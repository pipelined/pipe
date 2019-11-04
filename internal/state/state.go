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
		messages chan string
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
	merger struct {
		wg     *sync.WaitGroup
		errors chan error
	}

	// StartFunc is the closure to trigger the start of a pipe.
	StartFunc func(bufferSize int, cancel <-chan struct{}, messages chan<- string) []<-chan error

	// NewMessageFunc is the closure to send a message into a pipe.
	NewMessageFunc func(pipeID string)

	// PushParamsFunc is the closure to push new params into pipe.
	PushParamsFunc func(params Params)
)

type (
	// state identifies one of the possible states handle can be in.
	// It holds all channels that handle should listen to during this state.
	state struct {
		stateType
		events   <-chan event
		errors   <-chan error
		messages <-chan string
		params   <-chan Params
	}

	// stateType
	stateType uint
)

const (
	ready stateType = iota + 1
	running
	paused
	stopped
)

// NewHandle returns new initalized handle that can be used to manage lifecycle.
func NewHandle(start StartFunc, newMessage NewMessageFunc, pushParams PushParamsFunc) *Handle {
	h := Handle{
		events:       make(chan event, 1),
		params:       make(chan Params, 1),
		messages:     make(chan string),
		startFn:      start,
		newMessageFn: newMessage,
		pushParamsFn: pushParams,
	}
	return &h
}

// Loop listens until nil state is returned.
func Loop(h *Handle) {
	var (
		s = h.ready()
		t stateType
		f errors
	)
	for s.stateType != stopped {
		s, t, f = h.listen(s, t, f)
	}
	// close control channels
	close(h.events)
	close(h.params)
	close(f)
}

// idle is used to listen to handle's channels which are relevant for idle state.
// s is the new state, t is the target state and f is the channel to notify target transition.
func (h *Handle) listen(s state, t stateType, f errors) (state, stateType, errors) {
	// check if target state is reached
	if s.stateType == t {
		close(f)
		t = 0
	}
	var (
		err      error
		newState = state(s)
	)
	for {
		select {
		case e := <-s.events:
			newState, err = h.transition(s, e)
			if err != nil {
				// transition failed, notify.
				e.feedback() <- fmt.Errorf("%v transition during %v: %w", e, s, err)
			} else {
				// got new target.
				if t != 0 {
					close(f)
				}
				f = e.feedback()
				t = e.target()
			}
		case params := <-s.params:
			h.pushParamsFn(params)
		case pipeID := <-s.messages:
			h.newMessageFn(pipeID)
		case err, ok := <-s.errors:
			if ok {
				h.cancelFn()
				f <- fmt.Errorf("error during %v: %w", s, err)
			}
			return h.ready(), t, f
		}

		if s != newState {
			return newState, t, f
		}
	}
}

// ready states that the handle is ready and user
// can start it, send params or stop it.
func (h *Handle) ready() state {
	return state{
		stateType: ready,
		events:    h.events,
		params:    h.params,
	}
}

// running states that the handle is running and user
// can pause it, send params or stop it.
func (h *Handle) running() state {
	return state{
		stateType: running,
		events:    h.events,
		params:    h.params,
		messages:  h.messages,
		errors:    h.merger.errors,
	}
}

// paused states that the handle is paused and
// user can resume it, send params or stop it.
func (h *Handle) paused() state {
	return state{
		stateType: paused,
		events:    h.events,
		params:    h.params,
		errors:    h.merger.errors,
	}
}

// transition takes current state and incoming event to decide if
// new state should be reached. ErrInvalidState is returned if
// event cannot be processed in the current state.
func (h *Handle) transition(s state, e event) (state, error) {
	switch s.stateType {
	case ready:
		switch ev := e.(type) {
		case stop:
			return state{
				stateType: stopped,
				events:    s.events,
			}, nil
		case run:
			ctx, cancelFn := context.WithCancel(ev.Context)
			h.cancelFn = cancelFn
			h.merger = mergeErrors(h.startFn(ev.BufferSize, ctx.Done(), h.messages))
			return h.running(), nil
		}
	case running:
		switch e.(type) {
		case stop:
			return state{
				stateType: stopped,
			}, h.cancel()
		case pause:
			return h.paused(), nil
		}
	case paused:
		switch e.(type) {
		case stop:
			return state{
				stateType: stopped,
			}, h.cancel()
		case resume:
			return h.running(), nil
		}
	}
	return s, ErrInvalidState
}

func (s stateType) String() string {
	switch s {
	case ready:
		return "state.Ready"
	case running:
		return "state.Running"
	case paused:
		return "state.Paused"
	default:
		return "state.Unknown"
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

// done blocks until error is received or channel is closed.
func (m merger) done(ec <-chan error) {
	if err, ok := <-ec; ok {
		select {
		case m.errors <- err:
		default:
		}
	}
	m.wg.Done()
}
