package state

import "context"

type (
	// event triggers the state change.
	// Use imperative verbs for implementations.
	//
	// target identifies which idle state is expected after event is sent.
	// feedback is used to provide errors to the caller.
	event interface {
		target() idleState
		feedback() chan error
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

	// stop event is sent to stop the Handle.
	stop struct {
		errors
	}
)

// Feedback exposes error channel and used to satisfy event interface.
func (f errors) feedback() chan error {
	return f
}

// target() state of the Run event is Ready.
func (run) target() idleState {
	return Ready
}

// target() state of the Pause event is Paused.
func (pause) target() idleState {
	return Paused
}

// target() state of the Resume event is Ready.
func (resume) target() idleState {
	return Ready
}

// target() state of the Close event is nil.
func (stop) target() idleState {
	return nil
}
