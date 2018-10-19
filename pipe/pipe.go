package pipe

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/dudk/phono"
	"github.com/dudk/phono/log"
	"github.com/rs/xid"
)

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	name       string
	sampleRate phono.SampleRate
	phono.UID
	cancelFn   context.CancelFunc
	pump       *PumpRunner
	processors []*ProcessRunner
	sinks      []*SinkRunner

	// metrics holds all references to measurable components
	metrics map[string]Measurable

	params *phono.Params
	// errors channel
	errc chan error
	// event channel
	eventc chan eventMessage
	// transitions channel
	transitionc chan transitionMessage

	message struct {
		ask  chan struct{}
		take chan *phono.Message
	}

	log log.Logger
}

// Param provides a way to set parameters to pipe
// returns phono.ParamFunc, which can be executed later
type Param func(p *Pipe) phono.ParamFunc

type listenFn func() State

// State identifies one of the possible states pipe can be in
type State interface {
	listen(*Pipe) listenFn
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

// actionFn is an action function which causes a pipe state change
// chan error is closed when state is changed
type actionFn func(p *Pipe) (State, chan error)

// event identifies the type of event
type event int

// eventMessage is passed into pipe's event channel when user does some action
type eventMessage struct {
	event
	done      chan error
	params    *phono.Params
	callbacks []string
}

// transitionMessage is sent when pipe changes the state
type transitionMessage struct {
	State
	err error
}

// types of events
const (
	run event = iota
	pause
	resume
	params
	measure
)

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment
	ErrInvalidState = errors.New("Invalid state")
	// ErrEOP is returned if pump finished processing and indicates a gracefull ending
	ErrEOP = errors.New("End of pipe")
)

// New creates a new pipe and applies provided params
// returned pipe is in ready state
func New(sampleRate phono.SampleRate, params ...Param) *Pipe {
	p := &Pipe{
		sampleRate:  sampleRate,
		processors:  make([]*ProcessRunner, 0),
		sinks:       make([]*SinkRunner, 0),
		eventc:      make(chan eventMessage, 100),
		transitionc: make(chan transitionMessage, 100),
		log:         log.GetLogger(),
		metrics:     make(map[string]Measurable),
	}
	p.SetID(xid.New().String())
	for _, param := range params {
		param(p)()
	}
	go func() {
		var state State = Ready
		for state != nil {
			state = state.listen(p)()
		}
	}()
	return p
}

// WithName sets name to Pipe
func WithName(n string) Param {
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			p.name = n
		}
	}
}

// WithPump sets pump to Pipe
func WithPump(pump phono.Pump) Param {
	if pump.ID() == "" {
		pump.SetID(xid.New().String())
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			runner := NewPump(pump, NewMetric(pump.ID(), p.sampleRate))
			p.metrics[runner.ID()] = runner
			p.pump = runner
		}
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...phono.Processor) Param {
	for i := range processors {
		if processors[i].ID() == "" {
			processors[i].SetID(xid.New().String())
		}
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			for i := range processors {
				runner := NewProcessor(processors[i], NewMetric(processors[i].ID(), p.sampleRate))
				p.metrics[runner.ID()] = runner
				p.processors = append(p.processors, runner)
			}
		}
	}
}

// WithSinks sets sinks to Pipe
func WithSinks(sinks ...phono.Sink) Param {
	for i := range sinks {
		if sinks[i].ID() == "" {
			sinks[i].SetID(xid.New().String())
		}
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			for i := range sinks {
				runner := NewSink(sinks[i], NewMetric(sinks[i].ID(), p.sampleRate))
				p.metrics[runner.ID()] = runner
				p.sinks = append(p.sinks, runner)
			}
		}
	}
}

// Run sends a run event into pipe
func Run(p *Pipe) (State, chan error) {
	runEvent := eventMessage{
		event: run,
		done:  make(chan error),
	}
	p.eventc <- runEvent
	return Ready, runEvent.done
}

// Pause sends a pause event into pipe
func Pause(p *Pipe) (State, chan error) {
	pauseEvent := eventMessage{
		event: pause,
		done:  make(chan error),
	}
	p.eventc <- pauseEvent
	return Paused, pauseEvent.done
}

// Resume sends a resume event into pipe
func Resume(p *Pipe) (State, chan error) {
	resumeEvent := eventMessage{
		event: resume,
		done:  make(chan error),
	}
	p.eventc <- resumeEvent
	return Ready, resumeEvent.done
}

// Wait for state transition or first error to occur
func (p *Pipe) Wait(s State) error {
	for msg := range p.transitionc {
		if msg.err != nil {
			p.log.Debug(fmt.Sprintf("%v received signal: %v with error: %v", p, msg.State, msg.err))
			return msg.err
		}
		if msg.State == s {
			p.log.Debug(fmt.Sprintf("%v received state: %T", p, msg.State))
			return nil
		}
	}
	return nil
}

// WaitAsync allows to wait for state transition or first error occurance through the returned channel
func (p *Pipe) WaitAsync(s State) <-chan error {
	errc := make(chan error)
	go func() {
		err := p.Wait(s)
		if err != nil {
			errc <- err
		} else {
			close(errc)
		}
	}()
	return errc
}

// Push new params into pipe
func (p *Pipe) Push(newParams *phono.Params) {
	p.eventc <- eventMessage{
		event:  params,
		params: newParams,
	}
}

// Measure returns a buffered channel of Measure.
// Event is pushed into pipe to retrieve metrics. Metric measure is returned immediately if
// state is idle, otherwise it's returned once it's reached destination within pipe.
// Result channel has buffer sized to found components. Order of measures can differ from
// requested order due to pipeline configuration.
func (p *Pipe) Measure(ids ...string) <-chan Measure {
	if len(ids) == 0 {
		return nil
	}
	em := eventMessage{
		event:     measure,
		callbacks: make([]string, 0, len(ids)),
	}
	// check if passed ids are part of the pipe
	for _, id := range ids {
		_, ok := p.metrics[id]
		if ok {
			em.callbacks = append(em.callbacks, id)
		}
	}
	// no components found
	if len(em.callbacks) == 0 {
		return nil
	}
	// waitgroup to close metrics channel
	var wg sync.WaitGroup
	wg.Add(len(em.callbacks))
	// measures channel
	mc := make(chan Measure, len(em.callbacks))
	for _, id := range em.callbacks {
		m := p.metrics[id]
		param := phono.Param{
			ID: id,
			Apply: func() {
				mc <- m.Measure()
				wg.Done()
			},
		}
		em.params = em.params.Add(param)
	}
	//wait and close
	go func() {
		wg.Wait()
		close(mc)
	}()
	p.eventc <- em
	return mc
}

// Close must be called to clean up pipe's resources
func (p *Pipe) Close() {
	defer func() {
		recover()
	}()
	close(p.eventc)
}

// Begin executes a passed action and waits till first error or till the passed function receive a done signal
func (p *Pipe) Begin(fn actionFn) (State, error) {
	s, errc := fn(p)
	if errc == nil {
		return s, nil
	}
	for err := range errc {
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

// Do begins an action and waits for returned state
func (p *Pipe) Do(fn actionFn) error {
	s, errc := fn(p)
	if errc == nil {
		return p.Wait(s)
	}
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return p.Wait(s)
}

// merge error channels
func mergeErrors(errcList ...<-chan error) chan error {
	var wg sync.WaitGroup
	out := make(chan error, len(errcList))

	//function to wait for error channel
	output := func(c <-chan error) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(errcList))
	for _, e := range errcList {
		go output(e)
	}

	//wait and close out
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// broadcastToSinks passes messages to all sinks
func (p *Pipe) broadcastToSinks(in <-chan *phono.Message) ([]<-chan error, error) {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan *phono.Message, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan *phono.Message)
	}

	//start broadcast
	for i, s := range p.sinks {
		errc, err := s.Run(p.ID(), broadcasts[i])
		if err != nil {
			p.log.Debug(fmt.Sprintf("%v failed to start sink %v error: %v", p, s.ID(), err))
			return nil, err
		}
		errcList = append(errcList, errc)
	}

	go func() {
		//close broadcasts on return
		defer func() {
			for i := range broadcasts {
				close(broadcasts[i])
			}
		}()
		for buf := range in {
			for i := range broadcasts {
				broadcasts[i] <- buf
			}
		}
	}()

	return errcList, nil
}

// newMessage creates a new message with cached params
// if new params are pushed into pipe - next message will contain them
func (p *Pipe) newMessage() *phono.Message {
	m := new(phono.Message)
	if !p.params.Empty() {
		m.Params = p.params
		p.params = new(phono.Params)
	}
	return m
}

// soure returns a default message producer which will be sent to pump
func (p *Pipe) soure() phono.NewMessageFunc {
	p.message.ask = make(chan struct{})
	p.message.take = make(chan *phono.Message)
	var do struct{}
	// if pipe paused this call will block
	return func() *phono.Message {
		p.message.ask <- do
		msg := <-p.message.take
		msg.SourceID = p.ID()
		return msg
	}
}

// stop the pipe and clean up resources
// consequent calls do nothing
func (p *Pipe) close() {
	if p.cancelFn != nil {
		p.cancelFn()
	}
}

// Convert the event to a string
func (e event) String() string {
	switch e {
	case run:
		return "run"
	case pause:
		return "pause"
	case resume:
		return "resume"
	case params:
		return "params"
	}
	return "unknown"
}

// Convert pipe to string. If name is included if has value
func (p *Pipe) String() string {
	if p.name == "" {
		return p.ID()
	}
	return fmt.Sprintf("%v %v", p.name, p.ID())
}

// listenIdle is used to listen to pipe's channels which are relevant for idle state
func (p *Pipe) listenIdle(s idleState) listenFn {
	return func() State {
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
}

// listenActive is used to listen to pipe's channels which are relevant for active state
func (p *Pipe) listenActive(s activeState) listenFn {
	return func() State {
		var newState State
		for {
			select {
			case e, ok := <-p.eventc:
				if !ok {
					p.close()
					return nil
				}
				newState = s.transition(p, e)
			case <-p.message.ask:
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
}

func (s ready) listen(p *Pipe) listenFn {
	return p.listenIdle(s)
}

func (s ready) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case params:
		e.params.ApplyTo(p.ID())
		p.params = p.params.Merge(e.params)
		return s
	case measure:
		for _, id := range e.callbacks {
			e.params.ApplyTo(id)
		}
		return s
	case run:
		ctx, cancelFn := context.WithCancel(context.Background())
		p.cancelFn = cancelFn
		errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))

		// start pump
		out, errc, err := p.pump.Run(ctx, p.ID(), p.soure())
		if err != nil {
			p.log.Debug(fmt.Sprintf("%v failed to start pump %v error: %v", p, p.pump.ID(), err))
			e.done <- err
			p.cancelFn()
			return s
		}
		errcList = append(errcList, errc)

		// start chained processesing
		for _, proc := range p.processors {
			out, errc, err = proc.Run(p.ID(), out)
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

func (s running) listen(p *Pipe) listenFn {
	return p.listenActive(s)
}

func (s running) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case measure:
		fallthrough
	case params:
		e.params.ApplyTo(p.ID())
		p.params = p.params.Merge(e.params)
		return s
	case pause:
		close(e.done)
		return Pausing
	}
	e.done <- ErrInvalidState
	return s
}

func (s running) sendMessage(p *Pipe) State {
	p.message.take <- p.newMessage()
	return s
}

func (s running) handleError(p *Pipe, err error) State {
	if err != nil {
		p.transitionc <- transitionMessage{s, err}
		p.cancelFn()
	}
	return Ready
}

func (s pausing) listen(p *Pipe) listenFn {
	return p.listenActive(s)
}

func (s pausing) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case measure:
		fallthrough
	case params:
		e.params.ApplyTo(p.ID())
		p.params = p.params.Merge(e.params)
		return s
	}
	e.done <- ErrInvalidState
	return s
}

func (s pausing) sendMessage(p *Pipe) State {
	m := p.newMessage()
	var wg sync.WaitGroup
	wg.Add(len(p.sinks))
	for _, sink := range p.sinks {
		param := phono.ReceivedBy(&wg, sink)
		m.Params = m.Params.Add(param)
	}
	p.message.take <- m
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

func (s paused) listen(p *Pipe) listenFn {
	return p.listenIdle(s)
}

func (s paused) transition(p *Pipe, e eventMessage) State {
	switch e.event {
	case params:
		e.params.ApplyTo(p.ID())
		p.params = p.params.Merge(e.params)
		return s
	case measure:
		for _, id := range e.callbacks {
			e.params.ApplyTo(id)
		}
		return s
	case resume:
		close(e.done)
		return Running
	}
	e.done <- ErrInvalidState
	return s
}
