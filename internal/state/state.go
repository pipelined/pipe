package state

import (
	"errors"
	"sync"
)

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment.
	ErrInvalidState = errors.New("invalid state")
)

// Handle manages the lifecycle of the pipe.
type Handle struct {
	// Event channel used to handle new events for state machine.
	// created in constructor, closed when Handle is closed.
	Eventc chan Event
	// Params channel used to recieve new parameters.
	// created in constructor, closed when Handle is closed.
	Paramc chan Params
	// ask for new message request for the chain.
	// created in custructor, never closed.
	givec chan string
	// errc used to fan-in errors from components.
	// created in run event, closed when all components are done.
	merger
	// cancel the net execution.
	// created in run event, closed on cancel event or when error is recieved.
	cancelc      chan struct{}
	startFn      StartFunc
	newMessageFn NewMessageFunc
	pushParamsFn PushParamsFunc
}

// merger fans-in error channels.
type merger struct {
	wg   *sync.WaitGroup
	errc chan error
}

// StartFunc is the closure to trigger the start of a pipe.
type StartFunc func(bufferSize int, cancelc chan struct{}, givec chan<- string) []<-chan error

// NewMessageFunc is the closure to send a message into a pipe.
type NewMessageFunc func(pipeID string)

// PushParamsFunc is the closure to push new params into pipe.
type PushParamsFunc func(params Params)

// State identifies one of the possible states pipe can be in.
type State interface {
	listen(*Handle, State, Feedback) (State, State, Feedback)
	transition(*Handle, Event) (State, error)
}

// idleState identifies that the pipe is ONLY waiting for user to send an event.
type idleState interface {
	State
}

// activeState identifies that the pipe is processing signals and also is waiting for user to send an event.
type activeState interface {
	State
	sendMessage(h *Handle, pipeID string)
}

// states
type (
	ready   struct{}
	running struct{}
	paused  struct{}
)

// states variables
var (
	Ready   ready   // Ready means that pipe can be started.
	Running running // Running means that pipe is executing at the moment.
	Paused  paused  // Paused means that pipe is paused and can be resumed.
)

// Event triggers the state change.
// Use imperative verbs for implementations.
//
// Target identifies which idle state is expected after event is sent.
// Errc is used to provide feedback to the caller.
type Event interface {
	Target() State
	Errc() chan error
}

// Feedback is a wrapper for error channels. It's used to give feedback
// about state change or error occured during that change.
type Feedback chan error

// Errc exposes error channel and used to satisfy Event interface.
func (f Feedback) Errc() chan error {
	return f
}

// Run event is sent to start the run.
type Run struct {
	BufferSize int
	Feedback
}

// Target state of the Run event is Ready.
func (Run) Target() State {
	return Ready
}

// Pause event is sent to pause the run.
type Pause struct {
	Feedback
}

// Target state of the Pause event is Paused.
func (Pause) Target() State {
	return Paused
}

// Resume event is sent to resume the run.
type Resume struct {
	Feedback
}

// Target state of the Resume event is Ready.
func (Resume) Target() State {
	return Ready
}

// Close event is sent to close the Handle.
type Close struct {
	Feedback
}

// Target state of the Close event is nil.
func (Close) Target() State {
	return nil
}

// NewHandle returns new initalized handle that can be used to manage lifecycle.
func NewHandle(start StartFunc, newMessage NewMessageFunc, pushParams PushParamsFunc) *Handle {
	h := Handle{
		Eventc:       make(chan Event, 1),
		Paramc:       make(chan Params, 1),
		givec:        make(chan string),
		startFn:      start,
		newMessageFn: newMessage,
		pushParamsFn: pushParams,
	}
	return &h
}

// Loop listens until nil state is returned.
func Loop(h *Handle, s State) {
	var (
		t State
		f Feedback
	)
	for s != nil {
		s, t, f = s.listen(h, t, f)
	}
	// close control channels
	close(h.Eventc)
	close(h.Paramc)
	close(f)
}

// idle is used to listen to handle's channels which are relevant for idle state.
// s is the new state, t is the target state and d channel to notify target transition.
func (h *Handle) idle(s idleState, t State, f Feedback) (State, State, Feedback) {
	if s == t {
		f = f.dismiss()
	}
	var err error
	newState := State(s)
	for {
		select {
		case e := <-h.Eventc:
			newState, err = s.transition(h, e)
			if err != nil {
				handle(e.Errc(), err)
			} else {
				f.dismiss()
				f = e.Errc()
				t = e.Target()
			}
		case params := <-h.Paramc:
			h.pushParamsFn(params)
		}
		if s != newState {
			return newState, t, f
		}
	}
}

// active is used to listen to handle's channels which are relevant for active state.
func (h *Handle) active(s activeState, t State, f Feedback) (State, State, Feedback) {
	var err error
	newState := State(s)
	for {
		select {
		case e := <-h.Eventc:
			newState, err = s.transition(h, e)
			if err != nil {
				handle(e.Errc(), err)
			} else {
				f.dismiss()
				f = e.Errc()
				t = e.Target()
			}
		case params := <-h.Paramc:
			h.pushParamsFn(params)
		case pipeID := <-h.givec:
			s.sendMessage(h, pipeID)
		case err, ok := <-h.errc:
			if ok {
				close(h.cancelc)
				handle(f, err)
			}
			return Ready, t, f
		}
		if s != newState {
			return newState, t, f
		}
	}
}

func (h *Handle) cancel() error {
	close(h.cancelc)
	return wait(h.errc)
}

func (s ready) listen(h *Handle, t State, f Feedback) (State, State, Feedback) {
	return h.idle(s, t, f)
}

func (s ready) transition(h *Handle, e Event) (State, error) {
	switch ev := e.(type) {
	case Close:
		return nil, nil
	case Run:
		h.cancelc = make(chan struct{})
		h.merger = mergeErrors(h.startFn(ev.BufferSize, h.cancelc, h.givec))
		return Running, nil
	}
	return s, ErrInvalidState
}

func (s running) listen(h *Handle, t State, f Feedback) (State, State, Feedback) {
	return h.active(s, t, f)
}

func (s running) transition(h *Handle, e Event) (State, error) {
	switch e.(type) {
	case Close:
		return nil, h.cancel()
	case Pause:
		return Paused, nil
	}
	return s, ErrInvalidState
}

func (s running) sendMessage(h *Handle, pipeID string) {
	h.newMessageFn(pipeID)
}

func (s paused) listen(h *Handle, t State, f Feedback) (State, State, Feedback) {
	return h.idle(s, t, f)
}

func (s paused) transition(h *Handle, e Event) (State, error) {
	switch e.(type) {
	case Close:
		return nil, h.cancel()
	case Resume:
		return Running, nil
	}
	return s, ErrInvalidState
}

// reach closes error channel and cancel waiting of target.
func (f Feedback) dismiss() Feedback {
	if f != nil {
		close(f)
	}
	return nil
}

// handleError pushes error into target. panic happens if no target defined.
func handle(errc chan error, err error) {
	errc <- err
}

func wait(d <-chan error) error {
	for err := range d {
		if err != nil {
			return err
		}
	}
	return nil
}

// merge error channels from all components into one.
func mergeErrors(errcList []<-chan error) merger {
	m := merger{
		wg:   &sync.WaitGroup{},
		errc: make(chan error, 1),
	}

	//function to wait for error channel
	m.wg.Add(len(errcList))
	for _, ec := range errcList {
		go m.done(ec)
	}

	//wait and close out
	go func() {
		m.wg.Wait()
		close(m.errc)
	}()
	return m
}

func (m merger) done(ec <-chan error) {
	// block until error is received or channel is closed
	if err, ok := <-ec; ok {
		select {
		case m.errc <- err:
		default:
		}
	}
	m.wg.Done()
}
