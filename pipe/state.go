package pipe

import (
	"fmt"
	"sync"

	"github.com/dudk/phono"
)

// state identifies one of the possible states pipe can be in.
type state interface {
	listen(*Pipe, target) (state, target)
	transition(*Pipe, eventMessage) (state, error)
}

// idleState identifies that the pipe is ONLY waiting for user to send an event.
type idleState interface {
	state
}

// activeState identifies that the pipe is processing signals and also is waiting for user to send an event.
type activeState interface {
	state
	sendMessage(*Pipe) state
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
type actionFn func(p *Pipe) chan error

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
func (p *Pipe) Run() chan error {
	runEvent := eventMessage{
		event: run,
		target: target{
			state: ready,
			errc:  make(chan error, 1),
		},
	}
	p.events <- runEvent
	return runEvent.target.errc
}

// Pause sends a pause event into pipe.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Pause() chan error {
	pauseEvent := eventMessage{
		event: pause,
		target: target{
			state: paused,
			errc:  make(chan error, 1),
		},
	}
	p.events <- pauseEvent
	return pauseEvent.target.errc
}

// Resume sends a resume event into pipe.
// Calling this method after pipe is closed causes a panic.
func (p *Pipe) Resume() chan error {
	resumeEvent := eventMessage{
		event: resume,
		target: target{
			state: ready,
			errc:  make(chan error, 1),
		},
	}
	p.events <- resumeEvent
	return resumeEvent.target.errc
}

// Close must be called to clean up pipe's resources.
func (p *Pipe) Close() chan error {
	resumeEvent := eventMessage{
		event: cancel,
		target: target{
			state: ready,
			errc:  make(chan error, 1),
		},
	}
	p.events <- resumeEvent
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
func (p *Pipe) loop() {
	var s state = ready
	t := target{}
	for s != nil {
		s, t = s.listen(p, t)
		p.log.Debug(fmt.Sprintf("%v is %T", p, s))
	}
	// cancel last pending target
	t.dismiss()
	close(p.events)
}

// idle is used to listen to pipe's channels which are relevant for idle state.
// s is the new state, t is the target state and d channel to notify target transition.
func (p *Pipe) idle(s idleState, t target) (state, target) {
	if s == t.state || s == ready {
		t = t.dismiss()
	}
	for {
		var newState state
		var err error
		select {
		case e := <-p.events:
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
func (p *Pipe) active(s activeState, t target) (state, target) {
	for {
		var newState state
		var err error
		select {
		case e := <-p.events:
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
			return ready, t
		}
		if s != newState {
			return newState, t
		}
	}
}

func (s idleReady) listen(p *Pipe, t target) (state, target) {
	return p.idle(s, t)
}

func (s idleReady) transition(p *Pipe, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		interrupt(p.cancel)
		return nil, nil
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s, nil
	case measure:
		for _, id := range e.components {
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
		p.errc = mergeErrors(errcList...)
		return running, err
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
				select {
				case broadcasts[i] <- m:
				case <-p.cancel:
					return
				}
			}
		}
	}()

	return errcList, nil
}

// merge error channels from all components into one.
func mergeErrors(errcList ...<-chan error) (errc chan error) {
	var wg sync.WaitGroup
	errc = make(chan error, len(errcList))

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

	return
}

func (s activeRunning) listen(p *Pipe, t target) (state, target) {
	return p.active(s, t)
}

func (s activeRunning) transition(p *Pipe, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		interrupt(p.cancel)
		err := Wait(p.errc)
		return nil, err
	case measure:
		e.params.applyTo(p.ID())
		p.feedback = p.feedback.merge(e.params)
		return s, nil
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
		return s, nil
	case pause:
		return pausing, nil
	}
	return s, ErrInvalidState
}

func (s activeRunning) sendMessage(p *Pipe) state {
	p.consume <- p.newMessage()
	return s
}

func (s activePausing) listen(p *Pipe, t target) (state, target) {
	return p.active(s, t)
}

func (s activePausing) transition(p *Pipe, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		interrupt(p.cancel)
		err := Wait(p.errc)
		return nil, err
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

// send message with pause signal.
func (s activePausing) sendMessage(p *Pipe) state {
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
	return paused
}

func (s idlePaused) listen(p *Pipe, t target) (state, target) {
	return p.idle(s, t)
}

func (s idlePaused) transition(p *Pipe, e eventMessage) (state, error) {
	switch e.event {
	case cancel:
		interrupt(p.cancel)
		err := Wait(p.errc)
		return nil, err
	case push:
		e.params.applyTo(p.ID())
		p.params = p.params.merge(e.params)
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
	if t.state != nil {
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
