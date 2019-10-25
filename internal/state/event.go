package state

import (
	"context"
	"fmt"
)

type (
	// event triggers the state change.
	// Use imperative verbs for implementations.
	//
	// target identifies which idle state is expected after event is sent.
	// feedback is used to provide errors to the caller.
	event interface {
		target() idleState
		feedback() chan error
		fmt.Stringer
	}

	// errs is a wrapper for error channels. It's used to return errs
	// of state transition or error occured during that transition.
	errs chan error
)

type (
	// run event is sent to start the run.
	run struct {
		context.Context
		BufferSize int
		errs
	}

	// pause event is sent to pause the run.
	pause struct {
		errs
	}

	// resume event is sent to resume the run.
	resume struct {
		errs
	}

	// stop event is sent to stop the Handle.
	stop struct {
		errs
	}
)

// Feedback exposes error channel and used to satisfy event interface.
func (f errs) feedback() chan error {
	return f
}

// target() state of the Run event is Ready.
func (run) target() idleState {
	return Ready
}

func (run) String() string {
	return "event.Run"
}

// target() state of the Pause event is Paused.
func (pause) target() idleState {
	return Paused
}

func (pause) String() string {
	return "event.Pause"
}

// target() state of the Resume event is Ready.
func (resume) target() idleState {
	return Ready
}

func (resume) String() string {
	return "event.Resume"
}

// target() state of the Close event is nil.
func (stop) target() idleState {
	return nil
}

func (stop) String() string {
	return "event.Stop"
}
