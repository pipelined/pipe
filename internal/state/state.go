package state

import (
	"errors"
	"sync"
)

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment.
	ErrInvalidState = errors.New("invalid state")
)

type Handle struct {
	Eventc      chan EventMessage // event channel
	Errc        chan error        // errors channel
	NewMessagec chan string       // ask for new message request for the chain
	Cancel      chan struct{}     // cancellation channel
	StartFunc
	NewMessageFunc
	PushParamsFunc
}

type StartFunc func(bufferSize int, cancelc chan struct{}, provide chan<- string) []<-chan error

type NewMessageFunc func(pipeID string)

type PushParamsFunc func(componentID string, params Params)

// state identifies one of the possible states pipe can be in.
type state interface {
	listen(*Handle, target) (state, target)
	transition(*Handle, EventMessage) (state, error)
}

// idleState identifies that the pipe is ONLY waiting for user to send an event.
type idleState interface {
	state
}

// activeState identifies that the pipe is processing signals and also is waiting for user to send an event.
type activeState interface {
	state
	sendMessage(h *Handle, pipeID string) state
}

// states
type (
	idleReady     struct{}
	activeRunning struct{}
	// activePausing struct{}
	idlePaused struct{}
)

// states variables
var (
	ready   idleReady     // Ready means that pipe can be started.
	running activeRunning // Running means that pipe is executing at the moment.
	paused  idlePaused    // Paused means that pipe is paused and can be resumed.
	// pausing activePausing // Pausing means that pause event was sent, but still not reached all sinks.
)

// actionFn is an action function which causes a pipe state change
// chan error is closed when state is changed
type actionFn func(h *Handle) chan error

// event identifies the type of event
type event int

// EventMessage is passed into pipe's event channel when user does some action.
type EventMessage struct {
	event               // event type.
	Params              // new Params.
	components []string // ids of components which need to be called.
	target
	bufferSize int
}

// target identifies which state is expected from pipe.
type target struct {
	state idleState  // end state for this event.
	errc  chan error // channel to send errors. it's closed when target state is reached.
}

// types of events.
const (
	run event = iota
	pause
	resume
	push
	cancel
)

// Run sends a run event into pipe.
// Calling this method after pipe is closed causes a panic.
func (h *Handle) Run(bufferSize int) chan error {
	runEvent := EventMessage{
		event:      run,
		bufferSize: bufferSize,
		target: target{
			state: ready,
			errc:  make(chan error, 1),
		},
	}
	h.Eventc <- runEvent
	return runEvent.target.errc
}

// Pause sends a pause event into pipe.
// Calling this method after pipe is closed causes a panic.
func (h *Handle) Pause() chan error {
	pauseEvent := EventMessage{
		event: pause,
		target: target{
			state: paused,
			errc:  make(chan error, 1),
		},
	}
	h.Eventc <- pauseEvent
	return pauseEvent.target.errc
}

// Resume sends a resume event into pipe.
// Calling this method after pipe is closed causes a panic.
func (h *Handle) Resume() chan error {
	resumeEvent := EventMessage{
		event: resume,
		target: target{
			state: ready,
			errc:  make(chan error, 1),
		},
	}
	h.Eventc <- resumeEvent
	return resumeEvent.target.errc
}

// Close must be called to clean up pipe's resources.
func (h *Handle) Close() chan error {
	resumeEvent := EventMessage{
		event: cancel,
		target: target{
			state: nil,
			errc:  make(chan error, 1),
		},
	}
	h.Eventc <- resumeEvent
	return resumeEvent.target.errc
}

// Loop listens until nil state is returned.
func Loop(h *Handle) {
	var s state = ready
	t := target{}
	for s != nil {
		s, t = s.listen(h, t)
	}
	// cancel last pending target
	t.dismiss()
	close(h.Eventc)
}

// idle is used to listen to pipe's channels which are relevant for idle state.
// s is the new state, t is the target state and d channel to notify target transition.
func (h *Handle) idle(s idleState, t target) (state, target) {
	if s == t.state || s == ready {
		t = t.dismiss()
	}
	for {
		var newState state
		var err error
		select {
		case e := <-h.Eventc:
			newState, err = s.transition(h, e)
			if err != nil {
				e.target.handle(err)
			} else if e.hasTarget() {
				t.dismiss()
				t = e.target
			}
		}
		if s != newState {
			return newState, t
		}
	}
}

// active is used to listen to pipe's channels which are relevant for active state.
func (h *Handle) active(s activeState, t target) (state, target) {
	for {
		var newState state
		var err error
		select {
		case e := <-h.Eventc:
			newState, err = s.transition(h, e)
			if err != nil {
				e.target.handle(err)
			} else if e.hasTarget() {
				t.dismiss()
				t = e.target
			}
		case pipeID := <-h.NewMessagec:
			newState = s.sendMessage(h, pipeID)
		case err, ok := <-h.Errc:
			if ok {
				close(h.Cancel)
				t.handle(err)
			}
			return ready, t
		}
		if s != newState {
			return newState, t
		}
	}
}

func (s idleReady) listen(h *Handle, t target) (state, target) {
	return h.idle(s, t)
}

func (s idleReady) transition(h *Handle, e EventMessage) (state, error) {
	switch e.event {
	case cancel:
		return nil, nil
	case push:
		h.PushParamsFunc("", e.Params)
		return s, nil
	case run:
		h.Cancel = make(chan struct{})
		h.Errc = make(chan error)
		errcList := h.StartFunc(e.bufferSize, h.Cancel, h.NewMessagec)
		mergeErrors(h.Errc, errcList...)
		return running, nil
	}
	return s, ErrInvalidState
}

func (s activeRunning) listen(h *Handle, t target) (state, target) {
	return h.active(s, t)
}

func (s activeRunning) transition(h *Handle, e EventMessage) (state, error) {
	switch e.event {
	case cancel:
		close(h.Cancel)
		err := wait(h.Errc)
		return nil, err
	case push:
		// e.Params.applyTo(f.uid)
		h.PushParamsFunc("", e.Params)
		return s, nil
	case pause:
		return paused, nil
	}
	return s, ErrInvalidState
}

func (s activeRunning) sendMessage(h *Handle, pipeID string) state {
	h.NewMessageFunc(pipeID)
	return s
}

// func (s activePausing) listen(h *Handle, t target) (state, target) {
// 	return h.idle(s, t)
// }

// func (s activePausing) transition(h *Handle, e EventMessage) (state, error) {
// 	switch e.event {
// 	case cancel:
// 		close(h.Cancel)
// 		err := wait(h.Errc)
// 		return nil, err
// 	case push:
// 		h.PushParamsFunc("", e.Params)
// 		return s, nil
// 	}
// 	return s, ErrInvalidState
// }

// // send message with pause signal.
// func (s activePausing) sendMessage(h *Handle, pipeID string) state {
// 	return paused
// }

func (s idlePaused) listen(h *Handle, t target) (state, target) {
	return h.idle(s, t)
}

func (s idlePaused) transition(h *Handle, e EventMessage) (state, error) {
	switch e.event {
	case cancel:
		close(h.Cancel)
		err := wait(h.Errc)
		return nil, err
	case push:
		// e.Params.applyTo(f.uid)
		h.PushParamsFunc("", e.Params)
		return s, nil
	case resume:
		return running, nil
	}
	return s, ErrInvalidState
}

// hasTarget checks if event contaions target.
func (e EventMessage) hasTarget() bool {
	return e.target.errc != nil
}

// reach closes error channel and cancel waiting of target.
func (t target) dismiss() target {
	if t.errc != nil {
		t.state = nil
		close(t.errc)
		t.errc = nil
	}
	return t
}

// handleError pushes error into target. panic happens if no target defined.
func (t target) handle(err error) {
	if t.errc != nil {
		t.errc <- err
	} else {
		panic(err)
	}
}

// receivedBy returns channel which is closed when param received by identified entity
func receivedBy(wg *sync.WaitGroup, id string) func() {
	return func() {
		wg.Done()
	}
}

// merge error channels from all components into one.
func mergeErrors(errc chan<- error, errcList ...<-chan error) {
	var wg sync.WaitGroup

	//function to wait for error channel
	output := func(ec <-chan error) {
		for e := range ec {
			errc <- e
		}
		wg.Done()
	}
	wg.Add(len(errcList))
	for _, ec := range errcList {
		go output(ec)
	}

	//wait and close out
	go func() {
		wg.Wait()
		close(errc)
	}()
}

func wait(d chan error) error {
	for err := range d {
		if err != nil {
			return err
		}
	}
	return nil
}

// Convert the event to a string.
func (e event) String() string {
	switch e {
	case run:
		return "run"
	case pause:
		return "pause"
	case resume:
		return "resume"
	case push:
		return "params"
	}
	return "unknown"
}
