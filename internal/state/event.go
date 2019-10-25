package state

import "context"

type (
	// event triggers the state change.
	// Use imperative verbs for implementations.
	//
	// Target identifies which idle state is expected after event is sent.
	// Errc is used to provide feedback to the caller.
	event interface {
		Target() idleState
		Feedback() chan error
	}

	// Feedback is a wrapper for error channels. It's used to give feedback
	// about state change or error occured during that change.
	feedback chan error
)

type (
	// run event is sent to start the run.
	run struct {
		context.Context
		BufferSize int
		feedback
	}

	// pause event is sent to pause the run.
	pause struct {
		feedback
	}

	// resume event is sent to resume the run.
	resume struct {
		feedback
	}

	// stop event is sent to stop the Handle.
	stop struct {
		feedback
	}
)

// Feedback exposes error channel and used to satisfy event interface.
func (f feedback) Feedback() chan error {
	return f
}

// Target state of the Run event is Ready.
func (run) Target() idleState {
	return Ready
}

// Target state of the Pause event is Paused.
func (pause) Target() idleState {
	return Paused
}

// Target state of the Resume event is Ready.
func (resume) Target() idleState {
	return Ready
}

// Target state of the Close event is nil.
func (stop) Target() idleState {
	return nil
}