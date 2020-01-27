package state

import (
	"context"
	"fmt"
	"sync"

	"pipelined.dev/pipe/mutator"
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
		// errors used to fan-in errors from components.
		// created in run event, closed when all components are done.
		merger
		// cancel the line execution.
		// created in run event, closed on cancel event or when error is recieved.
		cancelFn context.CancelFunc
		startFn  StartFunc
		push     chan mutator.Mutators
		pull     chan chan mutator.Mutators
		mutators map[chan mutator.Mutators]mutator.Mutators
	}

	// merger fans-in error channels.
	merger struct {
		wg     *sync.WaitGroup
		errors chan error
	}

	// StartFunc is the closure to trigger the start of a pipe.
	StartFunc func(bufferSize int, cancel <-chan struct{}, puller chan<- chan mutator.Mutators) ([]<-chan error, error)

	// NewMessageFunc is the closure to send a message into a pipe.
	NewMessageFunc func(chan mutator.Mutators)

	// PushParamsFunc is the closure to push new mutators into pipe.
	PushParamsFunc func(mutators mutator.Mutators)
)

type (
	// state identifies one of the possible states handle can be in.
	// It holds all channels that handle should listen to during this state.
	state struct {
		stateType
		events <-chan event
		errors <-chan error
		pull   <-chan chan mutator.Mutators
		push   <-chan mutator.Mutators
	}

	// stateType
	stateType uint
)

const (
	undefined stateType = iota
	ready
	running
	paused
	interrupting
	closed
)

// NewHandle returns new initalized handle that can be used to manage lifecycle.
func NewHandle(start StartFunc) *Handle {
	h := Handle{
		startFn: start,
		events:  make(chan event, 1),
		push:    make(chan mutator.Mutators, 1),
	}
	return &h
}

// Loop listens until nil state is returned.
func Loop(h *Handle) {
	var (
		state    = h.ready()
		idle     = undefined
		feedback errors
	)
	// loop until done state is reached
	for state.stateType != closed {
		state, idle, feedback = h.listen(state, idle, feedback)
		// check if idle state is reached
		if state.stateType == idle {
			close(feedback)
			feedback = nil
			idle = undefined
		}
	}
}

// listen to handle's channels which are relevant for passed state.
// s is the new state, t is the idle state and f is the channel to notify idle transition.
func (h *Handle) listen(s state, idle stateType, f errors) (state, stateType, errors) {
	var (
		err     error
		current = s.stateType
	)
	for {
		select {
		case _ = <-s.push:
			panic("push mutators not implemented")
		case puller := <-s.pull:
			mutators := h.mutators[puller]
			puller <- mutators
			continue
		case e := <-s.events:
			s, err = h.transition(s, e)
			if err != nil {
				e.feedback() <- fmt.Errorf("%v transition during %v: %w", e, s, err)
				continue
			}

			// if we had previous feedback, dismiss.
			if f != nil {
				close(f)
			}
			// got new idle state.
			f = e.feedback()
			idle = e.idle()
		case err, ok := <-s.errors:
			if ok {
				h.cancelFn()
				// feedback has buffer of one error,
				// if more errors happen, they will be ignored.
				select {
				case f <- fmt.Errorf("error during %v: %w", s, err):
				default:
					// ignore if feedback buffer is full.
				}
			} else {
				s, _ = h.transition(s, done{})
			}
		}

		// return if state changed
		if s.stateType != current {
			return s, idle, f
		}
	}
}

// transition takes current state and incoming event to decide if
// new state should be reached. ErrInvalidState is returned if
// event cannot be processed in the current state.
func (h *Handle) transition(s state, e event) (state, error) {
	switch s.stateType {
	case ready:
		switch ev := e.(type) {
		case interrupt:
			// this is a special case as ready
			// state doesn't need an interruption.
			close(h.push)
			close(h.events)
			return h.closed(), nil
		case run:
			h.pull = make(chan chan mutator.Mutators)
			ctx, cancelFn := context.WithCancel(ev.Context)
			h.cancelFn = cancelFn
			errChans, err := h.startFn(ev.BufferSize, ctx.Done(), h.pull)
			if err != nil {
				return h.closed(), nil
			}
			h.merger = mergeErrors(errChans)
			return h.running(), nil
		}
	case running:
		switch e.(type) {
		case interrupt:
			return h.interrupting(), nil
		case pause:
			return h.paused(), nil
		case done:
			return h.ready(), nil
		}
	case paused:
		switch e.(type) {
		case interrupt:
			return h.interrupting(), nil
		case resume:
			return h.running(), nil
		case done:
			return h.ready(), nil
		}
	case interrupting:
		switch e.(type) {
		case done:
			return h.closed(), nil
		}
	}
	return s, ErrInvalidState
}

// ready states that the handle is ready and user
// can start it, send mutators or interrupt it.
func (h *Handle) ready() state {
	return state{
		stateType: ready,
		events:    h.events,
		push:      h.push,
	}
}

// running states that the handle is running and user
// can pause it, send mutators or interrupt it.
func (h *Handle) running() state {
	return state{
		stateType: running,
		events:    h.events,
		push:      h.push,
		pull:      h.pull,
		errors:    h.merger.errors,
	}
}

// paused states that the handle is paused and
// user can resume it, send mutators or interrupt it.
func (h *Handle) paused() state {
	return state{
		stateType: paused,
		events:    h.events,
		push:      h.push,
		errors:    h.merger.errors,
	}
}

// interrupting states that the handle is interrupting and
// user cannot do anything whith it anymore.
func (h *Handle) interrupting() state {
	h.cancelFn()
	close(h.push)
	close(h.events)
	return state{
		stateType: interrupting,
		errors:    h.merger.errors,
	}
}

func (h *Handle) closed() state {
	return state{stateType: closed}
}

func (s stateType) String() string {
	switch s {
	case ready:
		return "state.Ready"
	case running:
		return "state.Running"
	case paused:
		return "state.Paused"
	case interrupting:
		return "state.Interrupting"
	default:
		return "state.Unknown"
	}
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

func push(h *Handle) PushParamsFunc {
	return func(mutators mutator.Mutators) {
		// h.mutators = h.mutators.Append(mutators)
	}
}
