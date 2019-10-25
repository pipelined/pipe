package state

import (
	"context"
	"fmt"
	"sync"
)

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment.
	ErrInvalidState = fmt.Errorf("invalid state")
)

// Handle manages the lifecycle of the pipe.
type Handle struct {
	// event channel used to handle new events for state machine.
	// created in constructor, closed when Handle is closed.
	events chan event
	// Params channel used to recieve new parameters.
	// created in constructor, closed when Handle is closed.
	params chan Params
	// ask for new message request for the chain.
	// created in custructor, never closed.
	give chan string
	// errors used to fan-in errors from components.
	// created in run event, closed when all components are done.
	merger
	// cancel the line execution.
	// created in run event, closed on cancel event or when error is recieved.
	cancelFn     context.CancelFunc
	startFn      StartFunc
	newMessageFn NewMessageFunc
	pushParamsFn PushParamsFunc
}

// merger fans-in error channels.
type merger struct {
	wg     *sync.WaitGroup
	errors chan error
}

// StartFunc is the closure to trigger the start of a pipe.
type StartFunc func(bufferSize int, cancel <-chan struct{}, give chan<- string) []<-chan error

// NewMessageFunc is the closure to send a message into a pipe.
type NewMessageFunc func(pipeID string)

// PushParamsFunc is the closure to push new params into pipe.
type PushParamsFunc func(params Params)

// State identifies one of the possible states pipe can be in.
type State interface {
	listen(*Handle, State, errors) (State, idleState, errors)
	transition(*Handle, event) (State, error)
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

// NewHandle returns new initalized handle that can be used to manage lifecycle.
func NewHandle(start StartFunc, newMessage NewMessageFunc, pushParams PushParamsFunc) *Handle {
	h := Handle{
		events:       make(chan event, 1),
		params:       make(chan Params, 1),
		give:         make(chan string),
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
		f errors
	)
	for s != nil {
		s, t, f = s.listen(h, t, f)
	}
	// close control channels
	close(h.events)
	close(h.params)
	close(f)
}

// idle is used to listen to handle's channels which are relevant for idle state.
// s is the new state, t is the target state and f channel to notify target transition.
func (h *Handle) idle(s idleState, t idleState, f errors) (State, idleState, errors) {
	// check if target state is reached
	if s == t {
		close(f)
		t = nil
	}
	var (
		err      error
		newState = State(s)
	)
	for {
		select {
		case e := <-h.events:
			newState, err = s.transition(h, e)
			if err != nil {
				// transition failed, notify
				e.feedback() <- err
			} else {
				// got new target
				if t != nil {
					close(f)
				}
				// f.dismiss()
				f = e.feedback()
				t = e.target()
			}
		case params := <-h.params:
			h.pushParamsFn(params)
		}

		if s != newState {
			return newState, t, f
		}
	}
}

// active is used to listen to handle's channels which are relevant for active state.
func (h *Handle) active(s activeState, t State, f errors) (State, idleState, errors) {
	var (
		err      error
		newState = State(s)
	)
	for {
		select {
		case e := <-h.events:
			newState, err = s.transition(h, e)
			if err != nil {
				e.feedback() <- err
			} else {
				close(f)
				f = e.feedback()
				t = e.target()
			}
		case params := <-h.params:
			h.pushParamsFn(params)
		case pipeID := <-h.give:
			s.sendMessage(h, pipeID)
		case err, ok := <-h.errors:
			if ok {
				h.cancelFn()
				f <- err
			}
			return Ready, t, f
		}

		if s != newState {
			return newState, t, f
		}
	}
}

// Run sends a run event into handle.
func (h *Handle) Run(ctx context.Context, bufferSize int) chan error {
	errors := make(chan error, 1)
	h.events <- run{
		Context:    ctx,
		BufferSize: bufferSize,
		errors:     errors,
	}
	return errors
}

// Pause sends a pause event into handle.
func (h *Handle) Pause() chan error {
	errors := make(chan error, 1)
	h.events <- pause{
		errors: errors,
	}
	return errors
}

// Resume sends a resume event into handle.
func (h *Handle) Resume() chan error {
	errors := make(chan error, 1)
	h.events <- resume{
		errors: errors,
	}
	return errors
}

// Stop sends a stop event into handle.
func (h *Handle) Stop() chan error {
	errors := make(chan error, 1)
	h.events <- stop{
		errors: errors,
	}
	return errors
}

// Push new params into handle.
func (h *Handle) Push(params Params) {
	h.params <- params
}

func (h *Handle) cancel() error {
	h.cancelFn()
	return wait(h.errors)
}

func (s ready) listen(h *Handle, t State, f errors) (State, idleState, errors) {
	return h.idle(s, t, f)
}

func (s ready) transition(h *Handle, e event) (State, error) {
	switch ev := e.(type) {
	case stop:
		return nil, nil
	case run:
		ctx, cancelFn := context.WithCancel(ev.Context)
		h.cancelFn = cancelFn
		h.merger = mergeErrors(h.startFn(ev.BufferSize, ctx.Done(), h.give))
		return Running, nil
	}
	return s, ErrInvalidState
}

func (s running) listen(h *Handle, t State, f errors) (State, idleState, errors) {
	return h.active(s, t, f)
}

func (s running) transition(h *Handle, e event) (State, error) {
	switch e.(type) {
	case stop:
		return nil, h.cancel()
	case pause:
		return Paused, nil
	}
	return s, ErrInvalidState
}

func (s running) sendMessage(h *Handle, pipeID string) {
	h.newMessageFn(pipeID)
}

func (s paused) listen(h *Handle, t State, f errors) (State, idleState, errors) {
	return h.idle(s, t, f)
}

func (s paused) transition(h *Handle, e event) (State, error) {
	switch e.(type) {
	case stop:
		return nil, h.cancel()
	case resume:
		return Running, nil
	}
	return s, ErrInvalidState
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
		wg:     &sync.WaitGroup{},
		errors: make(chan error, 1),
	}

	//function to wait for error channel
	m.wg.Add(len(errcList))
	for _, ec := range errcList {
		go m.done(ec)
	}

	//wait and close out
	go func() {
		m.wg.Wait()
		close(m.errors)
	}()
	return m
}

func (m merger) done(ec <-chan error) {
	// block until error is received or channel is closed
	if err, ok := <-ec; ok {
		select {
		case m.errors <- err:
		default:
		}
	}
	m.wg.Done()
}
