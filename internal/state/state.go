package state

import (
	"context"
	"fmt"
	"sync"
)

var (
	// ErrInvalidState is returned if event cannot be handled at this state.
	ErrInvalidState = fmt.Errorf("invalid state")
)

type (
	// Handle manages the lifecycle of the pipe.
	Handle struct {
		// event channel used to handle new events for state machine.
		// created in constructor, closed when Handle is closed.
		events chan event
		// Params channel used to recieve new parameters.
		// created in constructor, closed when Handle is closed.
		params chan Params
		// ask for new message request for the chain.
		// created in custructor, never closed.
		give chan string
		// errs used to fan-in errs from components.
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
	merger struct {
		wg   *sync.WaitGroup
		errs chan error
	}

	// StartFunc is the closure to trigger the start of a pipe.
	StartFunc func(bufferSize int, cancel <-chan struct{}, give chan<- string) []<-chan error

	// NewMessageFunc is the closure to send a message into a pipe.
	NewMessageFunc func(pipeID string)

	// PushParamsFunc is the closure to push new params into pipe.
	PushParamsFunc func(params Params)
)

type (
	// state identifies one of the possible states pipe can be in.
	state interface {
		listen(*Handle, state, errs) (state, idleState, errs)
		transition(*Handle, event) (state, error)
		fmt.Stringer
	}

	// idleState identifies that the pipe is ONLY waiting for user to send an event.
	idleState interface {
		state
	}

	// activeState identifies that the pipe is processing signals and also is waiting for user to send an event.
	activeState interface {
		state
		sendMessage(h *Handle, pipeID string)
	}

	// states
	idleReady     struct{}
	activeRunning struct{}
	idlePaused    struct{}
)

// states variables
var (
	// Ready states that pipe can be started.
	ready idleReady
	// Running states that pipe is executing at the moment and can be paused.
	running activeRunning
	// Paused states that pipe is paused and can be resumed.
	paused idlePaused
)

// initial state for the handle.
var initialState = state(ready)

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
func Loop(h *Handle) {
	var (
		s = initialState
		t state
		f errs
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
// s is the new state, t is the target state and f is the channel to notify target transition.
func (h *Handle) idle(s idleState, t idleState, f errs) (state, idleState, errs) {
	// check if target state is reached
	if s == t {
		close(f)
		t = nil
	}
	var (
		err      error
		newState = state(s)
	)
	for {
		select {
		case e := <-h.events:
			newState, err = s.transition(h, e)
			if err != nil {
				// transition failed, notify.
				e.feedback() <- fmt.Errorf("%v transition during %v: %w", e, s, err)
			} else {
				// got new target.
				if t != nil {
					close(f)
				}
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
func (h *Handle) active(s activeState, t state, f errs) (state, idleState, errs) {
	var (
		err      error
		newState = state(s)
	)
	for {
		select {
		case e := <-h.events:
			newState, err = s.transition(h, e)
			if err != nil {
				// transition failed, notify.
				e.feedback() <- fmt.Errorf("%v transition during %v: %w", e, s, err)
			} else {
				// because active state always starts intentionally,
				// target and feedback are never nil.
				close(f)
				f = e.feedback()
				t = e.target()
			}
		case params := <-h.params:
			h.pushParamsFn(params)
		case pipeID := <-h.give:
			s.sendMessage(h, pipeID)
		case err, ok := <-h.errs:
			if ok {
				h.cancelFn()
				f <- fmt.Errorf("error during %v: %w", s, err)
			}
			return ready, t, f
		}

		if s != newState {
			return newState, t, f
		}
	}
}

// Run sends a run event into handle.
func (h *Handle) Run(ctx context.Context, bufferSize int) chan error {
	errs := make(chan error, 1)
	h.events <- run{
		Context:    ctx,
		BufferSize: bufferSize,
		errs:       errs,
	}
	return errs
}

// Pause sends a pause event into handle.
func (h *Handle) Pause() chan error {
	errs := make(chan error, 1)
	h.events <- pause{
		errs: errs,
	}
	return errs
}

// Resume sends a resume event into handle.
func (h *Handle) Resume() chan error {
	errs := make(chan error, 1)
	h.events <- resume{
		errs: errs,
	}
	return errs
}

// Stop sends a stop event into handle.
func (h *Handle) Stop() chan error {
	errs := make(chan error, 1)
	h.events <- stop{
		errs: errs,
	}
	return errs
}

// Push new params into handle.
func (h *Handle) Push(params Params) {
	h.params <- params
}

func (h *Handle) cancel() error {
	h.cancelFn()
	return wait(h.errs)
}

func (s idleReady) listen(h *Handle, t state, f errs) (state, idleState, errs) {
	return h.idle(s, t, f)
}

func (s idleReady) transition(h *Handle, e event) (state, error) {
	switch ev := e.(type) {
	case stop:
		return nil, nil
	case run:
		ctx, cancelFn := context.WithCancel(ev.Context)
		h.cancelFn = cancelFn
		h.merger = mergeErrors(h.startFn(ev.BufferSize, ctx.Done(), h.give))
		return running, nil
	}
	return s, ErrInvalidState
}

func (s idleReady) String() string {
	return "state.Ready"
}

func (s activeRunning) listen(h *Handle, t state, f errs) (state, idleState, errs) {
	return h.active(s, t, f)
}

func (s activeRunning) transition(h *Handle, e event) (state, error) {
	switch e.(type) {
	case stop:
		return nil, h.cancel()
	case pause:
		return paused, nil
	}
	return s, ErrInvalidState
}

func (s activeRunning) sendMessage(h *Handle, pipeID string) {
	h.newMessageFn(pipeID)
}

func (s activeRunning) String() string {
	return "state.Running"
}

func (s idlePaused) listen(h *Handle, t state, f errs) (state, idleState, errs) {
	return h.idle(s, t, f)
}

func (s idlePaused) transition(h *Handle, e event) (state, error) {
	switch e.(type) {
	case stop:
		return nil, h.cancel()
	case resume:
		return running, nil
	}
	return s, ErrInvalidState
}

func (s idlePaused) String() string {
	return "state.Paused"
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
		errs: make(chan error, 1),
	}

	//function to wait for error channel
	m.wg.Add(len(errcList))
	for _, ec := range errcList {
		go m.done(ec)
	}

	//wait and close out
	go func() {
		m.wg.Wait()
		close(m.errs)
	}()
	return m
}

// done blocks until error is received or channel is closed.
func (m merger) done(ec <-chan error) {
	if err, ok := <-ec; ok {
		select {
		case m.errs <- err:
		default:
		}
	}
	m.wg.Done()
}
