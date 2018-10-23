package pipe

import (
	"context"
	"fmt"
	"sync"

	"github.com/dudk/phono"
)

// State identifies one of the possible states pipe can be in
type State interface {
	listen(*Pipe) State
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
	// Ready [idle] state means that pipe can be started
	Ready ready

	// Running [active] state means that pipe is executing at the moment
	Running running

	// Paused [idle] state means that pipe is paused and can be resumed
	Paused paused

	// Pausing [active] state means that pause event was sent, but still not reached all sinks
	Pausing pausing
)

func (s ready) listen(p *Pipe) State {
	return p.idle(s)
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
			e.done <- err
			p.cancelFn()
			return s
		}
		errcList = append(errcList, errc)

		// start chained processesing
		for _, proc := range p.processors {
			out, errc, err = proc.run(p.ID(), out)
			if err != nil {
				p.log.Debug(fmt.Sprintf("%v failed to start processor %v error: %v", p, proc.ID(), err))
				e.done <- err
				p.cancelFn()
				return s
			}
			errcList = append(errcList, errc)
		}

		sinkErrcList, err := p.broadcastToSinks(out)
		if err != nil {
			e.done <- err
			p.cancelFn()
			return s
		}
		errcList = append(errcList, sinkErrcList...)
		p.errc = mergeErrors(errcList...)
		close(e.done)
		return Running
	}
	e.done <- ErrInvalidState
	return s
}

func (s running) listen(p *Pipe) State {
	return p.active(s)
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
		close(e.done)
		return Pausing
	}
	e.done <- ErrInvalidState
	return s
}

func (s running) sendMessage(p *Pipe) State {
	p.consumerc <- p.newMessage()
	return s
}

func (s running) handleError(p *Pipe, err error) State {
	if err != nil {
		p.transitionc <- transitionMessage{s, err}
		p.cancelFn()
	}
	return Ready
}

func (s pausing) listen(p *Pipe) State {
	return p.active(s)
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
	e.done <- ErrInvalidState
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
		param := phono.ReceivedBy(&wg, sink)
		m.feedback = m.feedback.add(param)
	}
	p.consumerc <- m
	wg.Wait()
	return Paused
}

func (s pausing) handleError(p *Pipe, err error) State {
	// if nil error is received, it means that pipe finished before pause got finished
	if err != nil {
		p.transitionc <- transitionMessage{Pausing, err}
		p.cancelFn()
	} else {
		// because pipe is finished, we need send this signal to stop waiting
		p.transitionc <- transitionMessage{Paused, nil}
	}
	return Ready
}

func (s paused) listen(p *Pipe) State {
	return p.idle(s)
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
		close(e.done)
		return Running
	}
	e.done <- ErrInvalidState
	return s
}

// idle is used to listen to pipe's channels which are relevant for idle state
func (p *Pipe) idle(s idleState) State {
	var newState State
	for {
		select {
		case e, ok := <-p.eventc:
			if !ok {
				p.close()
				return nil
			}
			newState = s.transition(p, e)
		}
		if s != newState {
			p.transitionc <- transitionMessage{newState, nil}
			return newState
		}
	}
}

// active is used to listen to pipe's channels which are relevant for active state
func (p *Pipe) active(s activeState) State {
	var newState State
	for {
		select {
		case e, ok := <-p.eventc:
			if !ok {
				p.close()
				return nil
			}
			newState = s.transition(p, e)
		case <-p.providerc:
			newState = s.sendMessage(p)
		case err := <-p.errc:
			newState = s.handleError(p, err)
		}
		if s != newState {
			p.transitionc <- transitionMessage{newState, nil}
			return newState
		}
	}
}
