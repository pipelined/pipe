package pipe

import (
	"sync"

	"github.com/dudk/phono"
)

// State identifies one of the possible states pipe can be in
type State interface {
	listen(*Pipe, target) (State, target)
	transition(*Pipe, eventMessage) (State, error)
}

// idleState identifies that the pipe is ONLY waiting for user to send an event
type idleState interface {
	State
}

// activeState identifies that the pipe is processing signals and also is waiting for user to send an event
type activeState interface {
	State
	sendMessage(*Pipe) State
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
	State idleState  // end state for this event.
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
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Run() chan error {
	runEvent := eventMessage{
		event: run,
		target: target{
			State: Ready,
			errc:  make(chan error, 1),
		},
	}
	p.eventc <- runEvent
	return runEvent.target.errc
}

// Pause sends a pause event into pipe.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Pause() chan error {
	pauseEvent := eventMessage{
		event: pause,
		target: target{
			State: Paused,
			errc:  make(chan error, 1),
		},
	}
	p.eventc <- pauseEvent
	return pauseEvent.target.errc
}

// Resume sends a resume event into pipe.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Resume() chan error {
	resumeEvent := eventMessage{
		event: resume,
		target: target{
			State: Ready,
			errc:  make(chan error, 1),
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
	if s == t.State || s == Ready {
		t = t.dismiss()
	}
	for {
		var newState State
		var err error
		select {
		case e, ok := <-p.eventc:
			if !ok {
				interrupt(p.cancel)
				return nil, t
			}
			newState, err = s.transition(p, e)
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
func (p *Pipe) active(s activeState, t target) (State, target) {
	for {
		var newState State
		var err error
		select {
		case e, ok := <-p.eventc:
			if !ok {
				interrupt(p.cancel)
				return nil, t
			}
			newState, err = s.transition(p, e)
			if err != nil {
				e.target.handle(err)
			} else if e.hasTarget() {
				t.dismiss()
				t = e.target
			}
		case <-p.provide:
			newState = s.sendMessage(p)
		case err, ok := <-p.errc:
			if ok {
				interrupt(p.cancel)
				t.handle(err)
			}
			return Ready, t
		}
		if s != newState {
			return newState, t
		}
	}
}

func (s ready) listen(p *Pipe, t target) (State, target) {
	return p.idle(s, t)
}

func (s ready) transition(p *Pipe, e eventMessage) (State, error) {
	switch e.event {
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s, nil
	case measure:
		for _, id := range e.callbacks {
			e.params.applyTo(id)
		}
		return s, nil
	case run:
		// build all runners first.
		// build pump.
		err := p.pump.build(p.ID())
		if err != nil {
			return s, err
		}

		// build processors.
		for _, proc := range p.processors {
			err := proc.build(p.ID())
			if err != nil {
				return s, err
			}
		}

		// build sinks.
		for _, sink := range p.sinks {
			err := sink.build(p.ID())
			if err != nil {
				return s, err
			}
		}

		errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))
		// start pump
		out, errc := p.pump.run(p.cancel, p.ID(), p.provide, p.consume)
		if err != nil {
			interrupt(p.cancel)
			return s, err
		}
		errcList = append(errcList, errc)

		// start chained processesing
		for _, proc := range p.processors {
			out, errc = proc.run(p.cancel, p.ID(), out)
			if err != nil {
				interrupt(p.cancel)
				return s, err
			}
			errcList = append(errcList, errc)
		}

		sinkErrcList, err := p.broadcastToSinks(out)
		if err != nil {
			interrupt(p.cancel)
			return s, err
		}
		errcList = append(errcList, sinkErrcList...)
		p.errc = mergeErrors(p.cancel, errcList...)
		return Running, err
	}
	return s, ErrInvalidState
}

// broadcastToSinks passes messages to all sinks.
func (p *Pipe) broadcastToSinks(in <-chan message) ([]<-chan error, error) {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan message, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan message)
	}

	//start broadcast
	for i, s := range p.sinks {
		errc := s.run(p.cancel, p.ID(), broadcasts[i])
		errcList = append(errcList, errc)
	}

	go func() {
		//close broadcasts on return
		defer func() {
			for i := range broadcasts {
				close(broadcasts[i])
			}
		}()
		for msg := range in {
			for i := range broadcasts {
				m := message{
					sourceID: msg.sourceID,
					Buffer:   msg.Buffer,
					params:   msg.params.detach(p.sinks[i].ID()),
					feedback: msg.feedback.detach(p.sinks[i].ID()),
				}
				broadcasts[i] <- m
			}
		}
	}()

	return errcList, nil
}

// merge error channels from all components into one.
func mergeErrors(cancel chan struct{}, errcList ...<-chan error) (errc chan error) {
	var wg sync.WaitGroup
	errc = make(chan error, len(errcList))

	//function to wait for error channel
	output := func(ec <-chan error) {
		select {
		case e, ok := <-ec:
			if ok {
				errc <- e
			}
		case <-cancel:
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

	return
}

func (s running) listen(p *Pipe, t target) (State, target) {
	return p.active(s, t)
}

func (s running) transition(p *Pipe, e eventMessage) (State, error) {
	switch e.event {
	case measure:
		e.params.applyTo(p.ID())
		p.feedback = p.feedback.merge(e.params)
		return s, nil
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s, nil
	case pause:
		return Pausing, nil
	}
	return s, ErrInvalidState
}

func (s running) sendMessage(p *Pipe) State {
	p.consume <- p.newMessage()
	return s
}

func (s pausing) listen(p *Pipe, t target) (State, target) {
	return p.active(s, t)
}

func (s pausing) transition(p *Pipe, e eventMessage) (State, error) {
	switch e.event {
	case measure:
		e.params.applyTo(p.ID())
		p.feedback = p.feedback.merge(e.params)
		return s, nil
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s, nil
	}
	return s, ErrInvalidState
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
	p.consume <- m
	wg.Wait()
	return Paused
}

func (s paused) listen(p *Pipe, t target) (State, target) {
	return p.idle(s, t)
}

func (s paused) transition(p *Pipe, e eventMessage) (State, error) {
	switch e.event {
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s, nil
	case measure:
		for _, id := range e.callbacks {
			e.params.applyTo(id)
		}
		return s, nil
	case resume:
		return Running, nil
	}
	return s, ErrInvalidState
}

// hasTarget checks if event contaions target.
func (e eventMessage) hasTarget() bool {
	return e.target.State != nil
}

// reach closes error channel and cancel waiting of target.
func (t target) dismiss() target {
	if t.State != nil {
		t.State = nil
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
