package state

import (
	"context"
	"fmt"
)

type (
	// event triggers the state change.
	// Use imperative verbs for implementations.
	//
	// idle identifies which idle state is expected after event is sent.
	// feedback is used to provide errors to the caller.
	event interface {
		idle() stateType
		feedback() chan error
		fmt.Stringer
	}

	// errors is a wrapper for error channels. It's used to return errors
	// of state transition or error occured during that transition.
	errors chan error
)

type (
	// run event is sent to start the run.
	run struct {
		context.Context
		BufferSize int
		errors
	}

	// pause event is sent to pause the run.
	pause struct {
		errors
	}

	// resume event is sent to resume the run.
	resume struct {
		errors
	}

	// interrupt event is sent to interrupt the Handle.
	interrupt struct {
		errors
	}
)

// Feedback exposes error channel and used to satisfy event interface.
func (f errors) feedback() chan error {
	return f
}

// idle state of the Run event is Ready.
func (run) idle() stateType {
	return ready
}

func (run) String() string {
	return "event.Run"
}

// idle state of the Pause event is Paused.
func (pause) idle() stateType {
	return paused
}

func (pause) String() string {
	return "event.Pause"
}

// idle state of the Resume event is Ready.
func (resume) idle() stateType {
	return running
}

func (resume) String() string {
	return "event.Resume"
}

// idle() state of the Close event is nil.
func (interrupt) idle() stateType {
	return done
}

func (interrupt) String() string {
	return "event.Interrupt"
}
