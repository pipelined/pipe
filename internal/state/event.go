package state

import (
	"context"
	"fmt"

	"pipelined.dev/pipe/mutator"
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

	// done event is sent when merger is done.
	// this event is not send by user.
	done struct{}
)

// Feedback exposes error channel and used to satisfy event interface.
func (f errors) feedback() chan error {
	return f
}

// Run sends a run event into handle.
// Calling this method after Interrupt, will cause panic.
func (h *Handle) Run(ctx context.Context) chan error {
	errors := make(chan error, 1)
	h.events <- run{
		Context: ctx,
		errors:  errors,
	}
	return errors
}

// Pause sends a pause event into handle.
// Calling this method after Interrupt, will cause panic.
func (h *Handle) Pause() chan error {
	errors := make(chan error, 1)
	h.events <- pause{
		errors: errors,
	}
	return errors
}

// Resume sends a resume event into handle.
// Calling this method after Interrupt, will cause panic.
func (h *Handle) Resume() chan error {
	errors := make(chan error, 1)
	h.events <- resume{
		errors: errors,
	}
	return errors
}

// Interrupt sends an interrupt event into handle.
func (h *Handle) Interrupt() chan error {
	errors := make(chan error, 1)
	h.events <- interrupt{
		errors: errors,
	}
	return errors
}

// Push new params into handle.
// Calling this method after Interrupt, will cause panic.
func (h *Handle) Push(mutators mutator.Mutators) {
	h.push <- mutators
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

// idle state of the Interrupt event is done.
func (interrupt) idle() stateType {
	return closed
}

func (interrupt) String() string {
	return "event.Interrupt"
}

func (done) idle() stateType {
	return closed
}

func (done) String() string {
	return "event.Done"
}

func (done) feedback() chan error {
	return nil
}
