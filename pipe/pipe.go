package pipe

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/dudk/phono"
)

// Pump is a source of samples
type Pump interface {
	Pump() phono.PumpFunc
}

// Processor defines interface for pipe-processors
type Processor interface {
	Process() phono.ProcessFunc
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink() phono.SinkFunc
}

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	cancelFn   context.CancelFunc
	pump       Pump
	processors []Processor
	sinks      []Sink

	cachedParams phono.Params
	// params channel
	paramc chan *phono.Params
	// errors channel
	errorc chan error
	// event channel
	eventc chan eventMessage
	// signals channel
	signalc chan signalMessage

	message struct {
		ask  chan struct{}
		take chan *phono.Message
	}
}

// Param provides a way to set parameters to pipe
// returns phono.ParamFunc, which can be executed later
type Param func(p *Pipe) phono.ParamFunc

// stateFn for pipe stateFn machine
type stateFn func(p *Pipe) stateFn

// event is sent when user does some action
type event int

type eventMessage struct {
	event
	done chan error
}

// Signal is sent when pipe changes the state
type Signal int

type signalMessage struct {
	Signal
	err error
}

const (
	run event = iota
	pause
	resume

	// Ready signal is sent when pipe goes to ready state
	Ready Signal = iota
	// Running signal is sent when pipe goes to running state
	Running
	// Paused signal is sent when pipe goes to paused state
	Paused
)

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment
	ErrInvalidState = errors.New("Invalid state")
)

// New creates a new pipe and applies provided params
// returned pipe is in ready state
func New(params ...Param) *Pipe {
	p := &Pipe{
		processors: make([]Processor, 0),
		sinks:      make([]Sink, 0),
		paramc:     make(chan *phono.Params),
		eventc:     make(chan eventMessage),
		signalc:    make(chan signalMessage, 100),
	}
	state := stateFn(ready)
	go func() {
		for state != nil {
			state = state(p)
		}
		p.stop()
	}()
	p.Wait(Ready)
	for _, param := range params {
		param(p)()
	}
	return p
}

// WithPump sets pump to Pipe
func WithPump(pump Pump) Param {
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			p.pump = pump
		}
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...Processor) Param {
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			p.processors = processors
		}
	}
}

// WithSinks sets sinks to Pipe
func WithSinks(sinks ...Sink) Param {
	return func(p *Pipe) phono.ParamFunc {
		return func() {
			p.sinks = sinks
		}
	}
}

// Run switches pipe into running state
func (p *Pipe) Run() chan error {
	runEvent := eventMessage{
		event: run,
		done:  make(chan error),
	}
	p.eventc <- runEvent
	return runEvent.done
}

// Pause switches pipe into pausing state
func (p *Pipe) Pause() chan error {
	pauseEvent := eventMessage{
		event: pause,
		done:  make(chan error),
	}
	p.eventc <- pauseEvent
	return pauseEvent.done
}

// Resume switches pipe into running state
func (p *Pipe) Resume() chan error {
	resumeEvent := eventMessage{
		event: resume,
		done:  make(chan error),
	}
	p.eventc <- resumeEvent
	return resumeEvent.done
}

// Wait for signal or first error
func (p *Pipe) Wait(s Signal) error {
	for msg := range p.signalc {
		if msg.err != nil {
			return msg.err
		}
		if msg.Signal == s {
			return nil
		}
	}
	return nil
}

// Do executes a passed action and waits till first error or till the passed function receive a done signal
func Do(fn func() chan error) error {
	errc := fn()
	if errc == nil {
		return nil
	}
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
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

func (p *Pipe) broadcastToSinks(ctx context.Context, in <-chan *phono.Message) ([]<-chan error, error) {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan *phono.Message, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan *phono.Message)
	}

	//start broadcast
	for i, s := range p.sinks {
		errc, err := s.Sink()(ctx, broadcasts[i])
		if err != nil {
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
		for in != nil {
			select {
			case buf, ok := <-in:
				if !ok {
					in = nil
				} else {
					for i := range broadcasts {
						broadcasts[i] <- buf
					}
				}
			case <-ctx.Done():
				return
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
		return <-p.message.take
	}
}

// ready state defines that pipe can:
// 	run - start processing
func ready(p *Pipe) stateFn {
	fmt.Println("pipe is ready")
	p.signalc <- signalMessage{Ready, nil}
	for {
		select {
		case newParams, ok := <-p.paramc:
			if !ok {
				return nil
			}
			newParams.ApplyTo(p)
			p.cachedParams = *p.cachedParams.Join(newParams)
		case e := <-p.eventc:
			fmt.Printf("ready new event: %v\n", e)
			switch e.event {
			case run:
				ctx, cancelFn := context.WithCancel(context.Background())
				p.cancelFn = cancelFn
				errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))

				// start pump
				out, errc, err := p.pump.Pump()(ctx, p.soure())
				if err != nil {
					e.done <- err
					p.cancelFn()
					return ready
				}
				errcList = append(errcList, errc)

				// start chained processesing
				for _, proc := range p.processors {
					out, errc, err = proc.Process()(ctx, out)
					if err != nil {
						e.done <- err
						p.cancelFn()
						return ready
					}
					errcList = append(errcList, errc)
				}

				sinkErrcList, err := p.broadcastToSinks(ctx, out)
				if err != nil {
					e.done <- err
					p.cancelFn()
					return ready
				}
				errcList = append(errcList, sinkErrcList...)
				p.errorc = mergeErrors(errcList...)
				close(e.done)
				return running
			default:
				e.done <- ErrInvalidState
			}
		}
	}
}

// running state defines that pipe can be:
//	paused - pause the processing
func running(p *Pipe) stateFn {
	fmt.Println("pipe is running")
	p.signalc <- signalMessage{Running, nil}
	for {
		select {
		case newParams, ok := <-p.paramc:
			if !ok {
				return nil
			}
			newParams.ApplyTo(p)
			p.cachedParams = *p.cachedParams.Join(newParams)
		case <-p.message.ask:
			message := new(phono.Message)
			if !p.cachedParams.Empty() {
				message.Params = &p.cachedParams
				p.cachedParams = phono.Params{}
			}
			p.message.take <- message
		case err := <-p.errorc:
			if err != nil {
				p.signalc <- signalMessage{Running, err}
				p.cancelFn()
			}
			return ready
		case e := <-p.eventc:
			fmt.Printf("running new event: %v\n", e)
			switch e.event {
			case pause:
				close(e.done)
				return pausing
			default:
				e.done <- ErrInvalidState
			}
		}
	}
}

// pausing defines a state when pipe accepted a pause command and pushed message with confirmation
func pausing(p *Pipe) stateFn {
	fmt.Println("pipe is pausing")
	for {
		select {
		case newParams, ok := <-p.paramc:
			if !ok {
				return nil
			}
			newParams.ApplyTo(p)
			p.cachedParams = *p.cachedParams.Join(newParams)
		case <-p.message.ask:
			message := new(phono.Message)
			if !p.cachedParams.Empty() {
				message.Params = &p.cachedParams
				p.cachedParams = phono.Params{}
			}
			message.WaitGroup = &sync.WaitGroup{}
			message.WaitGroup.Add(len(p.sinks))
			p.message.take <- message
			message.Wait()
			return paused
		case err := <-p.errorc:
			if err != nil {
				p.signalc <- signalMessage{Running, nil}
				p.cancelFn()
			}
			return ready
		}
	}
}

func paused(p *Pipe) stateFn {
	fmt.Println("pipe is paused")
	p.signalc <- signalMessage{Paused, nil}
	for {
		select {
		case newParams, ok := <-p.paramc:
			if !ok {
				return nil
			}
			newParams.ApplyTo(p)
			p.cachedParams = *p.cachedParams.Join(newParams)
		case e := <-p.eventc:
			fmt.Printf("paused new event: %v\n", e)
			switch e.event {
			case resume:
				close(e.done)
				return running
			default:
				e.done <- ErrInvalidState
			}
		}
	}
}

// stop the pipe and clean up resources
func (p *Pipe) stop() {
	close(p.message.ask)
	p.message.ask = nil
	close(p.message.take)
	p.message.take = nil
}

// Push new params into pipe
func (p *Pipe) Push(o *phono.Params) {
	p.paramc <- o
}

// Close must be called to clean up pipe's resources
func (p *Pipe) Close() {
	if p.cancelFn != nil {
		p.cancelFn()
	}
	close(p.paramc)
	p.paramc = nil
	close(p.eventc)
	p.eventc = nil
}
