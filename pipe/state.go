package pipe

import (
	"context"
	"fmt"
	"sync"

	"github.com/dudk/phono"
)

// State identifies one of the possible states pipe can be in
type State interface {
	listen(*Pipe, target) (State, target)
	transition(*Pipe, eventMessage) State
}

// idleState identifies that the pipe is ONLY waiting for user to send an event
type idleState interface {
	State
}

// activeState identifies that the pipe is processing signals and also is waiting for user to send an event
type activeState interface {
	State
	sendMessage(*Pipe) State
	handleError(*Pipe, error) State
}

// states
type (
	ready   struct{}
	running struct{}
	pausing struct{}
	paused  struct{}
)

// states variables
var (
	// Ready [idle] state means that pipe can be started.
	Ready ready

	// Running [active] state means that pipe is executing at the moment.
	Running running

	// Paused [idle] state means that pipe is paused and can be resumed.
	Paused paused

	// Pausing [active] state means that pause event was sent, but still not reached all sinks.
	Pausing pausing
)

// actionFn is an action function which causes a pipe state change
// chan error is closed when state is changed
type actionFn func(p *Pipe) chan error

// event identifies the type of event
type event int

// eventMessage is passed into pipe's event channel when user does some action.
type eventMessage struct {
	event              // event type.
	params    params   // new params.
	callbacks []string // ids of components which need to be called.
	target
}

// target identifies which state is expected from pipe.
type target struct {
	State            // end state for this event.
	errc  chan error // channel to send errors. it's closed when target state is reached.
}

// types of events.
const (
	run event = iota
	pause
	resume
	push
	measure
)

// Run sends a run event into pipe.
func (p *Pipe) Run() chan error {
	runEvent := eventMessage{
		event: run,
		target: target{
			State: Ready,
			errc:  make(chan error),
		},
	}
	p.eventc <- runEvent
	return runEvent.target.errc
}

// Pause sends a pause event into pipe.
func (p *Pipe) Pause() chan error {
	pauseEvent := eventMessage{
		event: pause,
		target: target{
			State: Paused,
			errc:  make(chan error),
		},
	}
	p.eventc <- pauseEvent
	return pauseEvent.target.errc
}

// Resume sends a resume event into pipe.
func (p *Pipe) Resume() chan error {
	resumeEvent := eventMessage{
		event: resume,
		target: target{
			State: Ready,
			errc:  make(chan error),
		},
	}
	p.eventc <- resumeEvent
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

// idle is used to listen to pipe's channels which are relevant for idle state.
// s is the new state, t is the target state and d channel to notify target transition.
func (p *Pipe) idle(s idleState, t target) (State, target) {
	if s == t.State {
		t = t.reach()
	}
	for {
		var newState State
		var e eventMessage
		var ok bool
		select {
		case e, ok = <-p.eventc:
			if !ok {
				p.close()
				return nil, t
			}
			newState = s.transition(p, e)
		}
		if e.hasTarget() {
			t.reach()
			t = e.target
		}
		if s != newState {
			return newState, t
		}
	}
}

// active is used to listen to pipe's channels which are relevant for active state.
func (p *Pipe) active(s activeState, t target) (State, target) {
	if s == t.State {
		t = t.reach()
	}
	for {
		var newState State
		var e eventMessage
		var ok bool
		select {
		case e, ok = <-p.eventc:
			if !ok {
				p.close()
				return nil, t
			}
			newState = s.transition(p, e)
		case <-p.providerc:
			newState = s.sendMessage(p)
		case err := <-p.errc:
			newState = s.handleError(p, err)
		}
		if e.hasTarget() {
			t.reach()
			t = e.target
		}
		if s != newState {
			return newState, t
		}
	}
}

func (s ready) listen(p *Pipe, t target) (State, target) {
	return p.idle(s, t)
}

func (s ready) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s
	case measure:
		for _, id := range e.callbacks {
			e.params.applyTo(id)
		}
		return s
	case run:
		ctx, cancelFn := context.WithCancel(context.Background())
		p.cancelFn = cancelFn
		errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))

		// start pump
		out, errc, err := p.pump.run(ctx, p.ID(), p.source())
		if err != nil {
			p.log.Debug(fmt.Sprintf("%v failed to start pump %v error: %v", p, p.pump.ID(), err))
			e.target.errc <- err
			p.cancelFn()
			return s
		}
		errcList = append(errcList, errc)

		// start chained processesing
		for _, proc := range p.processors {
			out, errc, err = proc.run(p.ID(), out)
			if err != nil {
				p.log.Debug(fmt.Sprintf("%v failed to start processor %v error: %v", p, proc.ID(), err))
				e.target.errc <- err
				p.cancelFn()
				return s
			}
			errcList = append(errcList, errc)
		}

		sinkErrcList, err := p.broadcastToSinks(out)
		if err != nil {
			e.target.errc <- err
			p.cancelFn()
			return s
		}
		errcList = append(errcList, sinkErrcList...)
		p.errc = mergeErrors(errcList...)
		return Running
	}
	e.target.errc <- ErrInvalidState
	return s
}

func (s running) listen(p *Pipe, t target) (State, target) {
	return p.active(s, t)
}

func (s running) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case measure:
		e.params.applyTo(p.ID())
		p.feedback = p.feedback.merge(e.params)
		return s
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s
	case pause:
		return Pausing
	}
	e.target.errc <- ErrInvalidState
	return s
}

func (s running) sendMessage(p *Pipe) State {
	p.consumerc <- p.newMessage()
	return s
}

func (s running) handleError(p *Pipe, err error) State {
	if err != nil {
		p.cancelFn()
	}
	return Ready
}

func (s pausing) listen(p *Pipe, t target) (State, target) {
	return p.active(s, t)
}

func (s pausing) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case measure:
		e.params.applyTo(p.ID())
		p.feedback = p.feedback.merge(e.params)
		return s
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s
	}
	e.target.errc <- ErrInvalidState
	return s
}

func (s pausing) sendMessage(p *Pipe) State {
	m := p.newMessage()
	if len(m.feedback) == 0 {
		m.feedback = make(map[string][]phono.ParamFunc)
	}
	var wg sync.WaitGroup
	wg.Add(len(p.sinks))
	for _, sink := range p.sinks {
		param := phono.ReceivedBy(&wg, sink.ID())
		m.feedback = m.feedback.add(param)
	}
	p.consumerc <- m
	wg.Wait()
	return Paused
}

func (s pausing) handleError(p *Pipe, err error) State {
	if err != nil {
		p.cancelFn()
	}
	return Ready
}

func (s paused) listen(p *Pipe, t target) (State, target) {
	return p.idle(s, t)
}

func (s paused) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s
	case measure:
		for _, id := range e.callbacks {
			e.params.applyTo(id)
		}
		return s
	case resume:
		return Running
	}
	e.target.errc <- ErrInvalidState
	return s
}

// hasTarget checks if event contaions target.
func (e eventMessage) hasTarget() bool {
	return e.target.State != nil
}

// reach closes error channel and cancel waiting of target.
func (t target) reach() target {
	if t.State != nil {
		t.State = nil
		close(t.errc)
		t.errc = nil
	}
	return t
}
