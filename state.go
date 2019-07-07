package pipe

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment.
	ErrInvalidState = errors.New("invalid state")
)

// state identifies one of the possible states pipe can be in.
type state interface {
	listen(*Flow, target) (state, target)
	transition(*Flow, eventMessage) (state, error)
}

// idleState identifies that the pipe is ONLY waiting for user to send an event.
type idleState interface {
	state
}

// activeState identifies that the pipe is processing signals and also is waiting for user to send an event.
type activeState interface {
	state
	sendMessage(f *Flow, pipeID string) state
}

// states
type (
	idleReady     struct{}
	activeRunning struct{}
	activePausing struct{}
	idlePaused    struct{}
)

// states variables
var (
	ready   idleReady     // Ready means that pipe can be started.
	running activeRunning // Running means that pipe is executing at the moment.
	paused  idlePaused    // Paused means that pipe is paused and can be resumed.
	pausing activePausing // Pausing means that pause event was sent, but still not reached all sinks.
)

// actionFn is an action function which causes a pipe state change
// chan error is closed when state is changed
type actionFn func(f *Flow) chan error

// event identifies the type of event
type event int

// eventMessage is passed into pipe's event channel when user does some action.
type eventMessage struct {
	event               // event type.
	params     params   // new params.
	components []string // ids of components which need to be called.
	target
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
	measure
	cancel
)

// Run sends a run event into pipe.
// Calling this method after pipe is closed causes a panic.
func (f *Flow) Run() chan error {
	runEvent := eventMessage{
		event: run,
		target: target{
			state: ready,
			errc:  make(chan error, 1),
		},
	}
	f.events <- runEvent
	return runEvent.target.errc
}

// Pause sends a pause event into pipe.
// Calling this method after pipe is closed causes a panic.
func (f *Flow) Pause() chan error {
	pauseEvent := eventMessage{
		event: pause,
		target: target{
			state: paused,
			errc:  make(chan error, 1),
		},
	}
	f.events <- pauseEvent
	return pauseEvent.target.errc
}

// Resume sends a resume event into pipe.
// Calling this method after pipe is closed causes a panic.
func (f *Flow) Resume() chan error {
	resumeEvent := eventMessage{
		event: resume,
		target: target{
			state: ready,
			errc:  make(chan error, 1),
		},
	}
	f.events <- resumeEvent
	return resumeEvent.target.errc
}

// Close must be called to clean up pipe's resources.
func (f *Flow) Close() chan error {
	resumeEvent := eventMessage{
		event: cancel,
		target: target{
			state: nil,
			errc:  make(chan error, 1),
		},
	}
	f.events <- resumeEvent
	return resumeEvent.target.errc
}

// Wait for state transition or first error to occur.
func Wait(d chan error) error {
	for err := range d {
		if err != nil {
			return err
		}
	}
	return nil
}

// loop listens until nil state is returned.
func loop(f *Flow) {
	var s state = ready
	t := target{}
	for s != nil {
		s, t = s.listen(f, t)
		f.log.Debug(fmt.Sprintf("%v is %T", f, s))
	}
	// cancel last pending target
	t.dismiss()
	close(f.events)
}

// idle is used to listen to pipe's channels which are relevant for idle state.
// s is the new state, t is the target state and d channel to notify target transition.
func (f *Flow) idle(s idleState, t target) (state, target) {
	if s == t.state || s == ready {
		t = t.dismiss()
	}
	for {
		var newState state
		var err error
		select {
		case e := <-f.events:
			newState, err = s.transition(f, e)
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
func (f *Flow) active(s activeState, t target) (state, target) {
	for {
		var newState state
		var err error
		select {
		case e := <-f.events:
			newState, err = s.transition(f, e)
			if err != nil {
				e.target.handle(err)
			} else if e.hasTarget() {
				t.dismiss()
				t = e.target
			}
		case pipeID := <-f.provide:
			newState = s.sendMessage(f, pipeID)
		case err, ok := <-f.errc:
			if ok {
				interrupt(f.cancel)
				t.handle(err)
			}
			return ready, t
		}
		if s != newState {
			return newState, t
		}
	}
}

func (s idleReady) listen(f *Flow, t target) (state, target) {
	return f.idle(s, t)
}

func (s idleReady) transition(f *Flow, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		return nil, nil
	case push:
		// e.params.applyTo(f.uid)
		f.params = f.params.merge(e.params)
		return s, nil
	case measure:
		for _, id := range e.components {
			e.params.applyTo(id)
		}
		return s, nil
	case run:
		f.cancel = make(chan struct{})
		var errcList []<-chan error
		for _, c := range f.chains {
			errcList = append(errcList, start(c, f.cancel, f.provide)...)
		}
		f.errc = mergeErrors(errcList...)
		return running, nil
	}
	return s, ErrInvalidState
}

func (s activeRunning) listen(f *Flow, t target) (state, target) {
	return f.active(s, t)
}

func (s activeRunning) transition(f *Flow, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		interrupt(f.cancel)
		err := Wait(f.errc)
		return nil, err
	case measure:
		// e.params.applyTo(f.uid)
		f.feedback = f.feedback.merge(e.params)
		return s, nil
	case push:
		// e.params.applyTo(f.uid)
		f.params = f.params.merge(e.params)
		return s, nil
	case pause:
		return pausing, nil
	}
	return s, ErrInvalidState
}

func (s activeRunning) sendMessage(f *Flow, pipeID string) state {
	c := f.chains[pipeID]
	c.consume <- newMessage(f, c)
	return s
}

// newMessage creates a new message with cached params.
// if new params are pushed into pipe - next message will contain them.
func newMessage(f *Flow, c chain) message {
	m := message{sourceID: c.uid}
	if len(f.params) > 0 {
		m.params = f.params
		f.params = make(map[string][]func())
	}
	if len(f.feedback) > 0 {
		m.feedback = f.feedback
		f.feedback = make(map[string][]func())
	}
	return m
}

func (s activePausing) listen(f *Flow, t target) (state, target) {
	return f.active(s, t)
}

func (s activePausing) transition(f *Flow, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		interrupt(f.cancel)
		err := Wait(f.errc)
		return nil, err
	case measure:
		// e.params.applyTo(f.uid)
		f.feedback = f.feedback.merge(e.params)
		return s, nil
	case push:
		// e.params.applyTo(f.uid)
		f.params = f.params.merge(e.params)
		return s, nil
	}
	return s, ErrInvalidState
}

// send message with pause signal.
func (s activePausing) sendMessage(f *Flow, pipeID string) state {
	c := f.chains[pipeID]
	m := newMessage(f, c)
	if len(m.feedback) == 0 {
		m.feedback = make(map[string][]func())
	}
	var wg sync.WaitGroup
	wg.Add(len(c.sinks))
	for _, sink := range c.sinks {
		uid := c.components[sink]
		param := receivedBy(&wg, uid)
		m.feedback = m.feedback.add(uid, param)
	}
	c.consume <- m
	wg.Wait()
	return paused
}

func (s idlePaused) listen(f *Flow, t target) (state, target) {
	return f.idle(s, t)
}

func (s idlePaused) transition(f *Flow, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		interrupt(f.cancel)
		err := Wait(f.errc)
		return nil, err
	case push:
		// e.params.applyTo(f.uid)
		f.params = f.params.merge(e.params)
		return s, nil
	case measure:
		for _, id := range e.components {
			e.params.applyTo(id)
		}
		return s, nil
	case resume:
		return running, nil
	}
	return s, ErrInvalidState
}

// hasTarget checks if event contaions target.
func (e eventMessage) hasTarget() bool {
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

// interrupt the pipe and clean up resources.
// consequent calls do nothing.
func interrupt(cancel chan struct{}) {
	close(cancel)
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
