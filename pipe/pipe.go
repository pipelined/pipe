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

// Pump is a source of samples
type Pump interface {
	phono.Identifiable
	RunPump(sourceID string) PumpRunner
	Pump(*phono.Message) (*phono.Message, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	phono.Identifiable
	RunProcess(sourceID string) ProcessRunner
	Process(*phono.Message) (*phono.Message, error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	phono.Identifiable
	RunSink(sourceID string) SinkRunner
	Sink(*phono.Message) error
}

// PumpRunner is a pump runner
type PumpRunner interface {
	phono.Identifiable
	Run(context.Context, phono.NewMessageFunc) (<-chan *phono.Message, <-chan error, error)
}

// ProcessRunner is a processor runner
type ProcessRunner interface {
	phono.Identifiable
	Run(<-chan *phono.Message) (<-chan *phono.Message, <-chan error, error)
}

// SinkRunner is a sink runner
type SinkRunner interface {
	phono.Identifiable
	Run(in <-chan *phono.Message) (errc <-chan error, err error)
}

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	name string
	phono.UID
	cancelFn   context.CancelFunc
	pump       PumpRunner
	processors []ProcessRunner
	sinks      []SinkRunner

	cachedParams *phono.Params
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

// stateFn is the state function for pipe state machine
type stateFn func(p *Pipe) State

// actionFn is an action function which causes a pipe state change
// chan error is closed when state is changed
type actionFn func(p *Pipe) (State, chan error)

// event is sent when user does some action
type event int

type eventMessage struct {
	event
	done   chan error
	params *phono.Params
}

// State is sent when pipe changes the state
type State int

type transitionMessage struct {
	State
	err error
}

const (
	run event = iota
	pause
	resume
	params
)

const (
	initial State = iota
	// Ready state means that pipe can be started
	Ready
	// Running state means that pipe is executing at the moment
	Running
	// Paused state means that pipe is paused and can be resumed
	Paused
	// Pausing state means that signal for pause was sent, but still not reached all sinks
	Pausing
	// closed state means that pipe execution was finished and it cannot be used anymore
	closed
)

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment
	ErrInvalidState = errors.New("Invalid state")
	// ErrEOP is returned if pump finished processing and indicates a gracefull ending
	ErrEOP = errors.New("End of pipe")
)

// New creates a new pipe and applies provided params
// returned pipe is in ready state
func New(params ...Param) *Pipe {
	p := &Pipe{
		processors:  make([]ProcessRunner, 0),
		sinks:       make([]SinkRunner, 0),
		eventc:      make(chan eventMessage),
		transitionc: make(chan transitionMessage, 100),
		log:         log.GetLogger(),
	}
	p.SetID(xid.New().String())
	for _, param := range params {
		param(p)()
	}
	state := initial
	newState := Ready
	go func() {
		defer p.stop()
		for {
			p.log.Debug(fmt.Sprintf("Current state: %v New State: %v\n", state, newState))
			state = state.transit(newState, p)
			if state == closed {
				return
			}
			newState = state.fn()(p)
		}
	}()
	p.Wait(Ready)
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
func WithPump(pump Pump) Param {
	if pump.ID() == "" {
		pump.SetID(xid.New().String())
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			p.pump = pump.RunPump(p.ID())
		}
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...Processor) Param {
	for i := range processors {
		if processors[i].ID() == "" {
			processors[i].SetID(xid.New().String())
		}
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			for i := range processors {
				p.processors = append(p.processors, processors[i].RunProcess(p.ID()))
			}
		}
	}
}

// WithSinks sets sinks to Pipe
func WithSinks(sinks ...Sink) Param {
	for i := range sinks {
		if sinks[i].ID() == "" {
			sinks[i].SetID(xid.New().String())
		}
	}
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			for i := range sinks {
				p.sinks = append(p.sinks, sinks[i].RunSink(p.ID()))
			}
		}
	}
}

// Run switches pipe into running state
func Run(p *Pipe) (State, chan error) {
	runEvent := eventMessage{
		event: run,
		done:  make(chan error),
	}
	p.eventc <- runEvent
	return Ready, runEvent.done
}

// Pause switches pipe into pausing state
func Pause(p *Pipe) (State, chan error) {
	pauseEvent := eventMessage{
		event: pause,
		done:  make(chan error),
	}
	p.eventc <- pauseEvent
	return Paused, pauseEvent.done
}

// Resume switches pipe into running state
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
			p.log.Debug(fmt.Sprintf("%v received signal: %v", p, msg.State))
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

// Close must be called to clean up pipe's resources
func (p *Pipe) Close() {
	if p.cancelFn != nil {
		p.cancelFn()
	}
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
		errc, err := s.Run(broadcasts[i])
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

// soure returns a default message producer which caches params
// if new params are pushed into pipe - next message will contain them
// if pipe paused this call will block
func (p *Pipe) soure() phono.NewMessageFunc {
	p.message.ask = make(chan struct{})
	p.message.take = make(chan *phono.Message)
	var do struct{}
	return func() *phono.Message {
		p.message.ask <- do
		msg := <-p.message.take
		msg.SourceID = p.ID()
		return msg
	}
}

// ready state defines that pipe can:
// 	run - start processing
func ready(p *Pipe) State {
	p.log.Debug(fmt.Sprintf("%v is ready", p))
	// p.transitionc <- transitionMessage{Ready, nil}
	for {
		select {
		case e, ok := <-p.eventc:
			if !ok {
				return closed
			}
			return Ready.handle(e, p)
		}
	}
}

// running state defines that pipe can be:
//	paused - pause the processing
func running(p *Pipe) State {
	p.log.Debug(fmt.Sprintf("%v is running", p))
	// p.transitionc <- transitionMessage{Running, nil}
	for {
		select {
		case <-p.message.ask:
			message := new(phono.Message)
			if !p.cachedParams.Empty() {
				message.Params = p.cachedParams
				p.cachedParams = &phono.Params{}
			}
			p.message.take <- message
		case err := <-p.errc:
			if err != nil {
				p.transitionc <- transitionMessage{Running, err}
				p.cancelFn()
			}
			return Ready
		case e, ok := <-p.eventc:
			if !ok {
				return closed
			}
			return Running.handle(e, p)
		}
	}
}

// pausing defines a state when pipe accepted a pause command and pushed message with confirmation
func pausing(p *Pipe) State {
	p.log.Debug(fmt.Sprintf("%v is pausing", p))
	for {
		select {
		case <-p.message.ask:
			message := new(phono.Message)
			if !p.cachedParams.Empty() {
				message.Params = p.cachedParams
				p.cachedParams = &phono.Params{}
			}
			var wg sync.WaitGroup
			wg.Add(len(p.sinks))
			for _, sink := range p.sinks {
				param := phono.ReceivedBy(&wg, sink)
				message.Params = message.Params.Add(param)
			}
			p.message.take <- message
			wg.Wait()
			return Paused
		case err := <-p.errc:
			// if nil error is received, it means that pipe finished before pause got finished
			if err != nil {
				p.transitionc <- transitionMessage{Pausing, err}
				p.cancelFn()
			} else {
				// because pipe is finished, we need send this signal to stop waiting
				p.transitionc <- transitionMessage{Paused, nil}
			}
			return Ready
		case e, ok := <-p.eventc:
			if !ok {
				return closed
			}
			return Pausing.handle(e, p)
		}
	}
}

func paused(p *Pipe) State {
	p.log.Debug(fmt.Sprintf("%v is paused", p))
	// p.transitionc <- transitionMessage{Paused, nil}
	for {
		select {
		case e, ok := <-p.eventc:
			if !ok {
				return closed
			}
			p.log.Debug(fmt.Sprintf("%v got new event: %v", p, e))
			return Paused.handle(e, p)
		}
	}
}

func (s State) fn() stateFn {
	switch s {
	case Ready:
		return ready
	case Pausing:
		return pausing
	case Paused:
		return paused
	case Running:
		return running
	}
	return nil
}

func (s State) transit(newState State, p *Pipe) State {
	if s != newState {
		p.transitionc <- transitionMessage{newState, nil}
	}
	return newState
}

func (s State) handle(e eventMessage, p *Pipe) State {
	switch e.event {
	case params:
		e.params.ApplyTo(p)
		p.cachedParams = p.cachedParams.Merge(e.params)
		return s
	case run:
		if s == Ready {
			ctx, cancelFn := context.WithCancel(context.Background())
			p.cancelFn = cancelFn
			errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))

			// start pump
			// pumpRunner := p.pump.RunPump(p.ID())
			out, errc, err := p.pump.Run(ctx, p.soure())
			if err != nil {
				p.log.Debug(fmt.Sprintf("%v failed to start pump %v error: %v", p, p.pump.ID(), err))
				e.done <- err
				p.cancelFn()
				return Ready
			}
			errcList = append(errcList, errc)

			// start chained processesing
			for _, proc := range p.processors {
				out, errc, err = proc.Run(out)
				if err != nil {
					p.log.Debug(fmt.Sprintf("%v failed to start processor %v error: %v", p, proc.ID(), err))
					e.done <- err
					p.cancelFn()
					return Ready
				}
				errcList = append(errcList, errc)
			}

			sinkErrcList, err := p.broadcastToSinks(out)
			if err != nil {
				e.done <- err
				p.cancelFn()
				return Ready
			}
			errcList = append(errcList, sinkErrcList...)
			p.errc = mergeErrors(errcList...)
			close(e.done)
			return Running
		}
	case pause:
		if s == Running {
			close(e.done)
			return Pausing
		}
	case resume:
		if s == Paused {
			close(e.done)
			return Running
		}
	}
	e.done <- ErrInvalidState
	return s
}

// stop the pipe and clean up resources
func (p *Pipe) stop() {
	p.log.Debug("Stop is called!")

	close(p.message.ask)
	close(p.message.take)
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

// Convert the event to a string
func (s State) String() string {
	switch s {
	case initial:
		return "initial"
	case Ready:
		return "ready"
	case Running:
		return "running"
	case Paused:
		return "paused"
	case Pausing:
		return "pausing"
	case closed:
		return "closed"
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
