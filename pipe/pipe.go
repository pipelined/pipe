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
	Validate() error
	Pump() phono.PumpFunc
}

// Processor defines interface for pipe-processors
type Processor interface {
	Validate() error
	Process() phono.ProcessFunc
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Validate() error
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
	eventc chan event

	// TODO: replace with single channel
	Signal struct {
		// closed when pipe goes into ready state
		Ready chan error
		// closed when pipe goes into running state
		Running chan error
		// closed when pipe goes paused
		Paused chan error
	}
	message struct {
		ask  chan struct{}
		take chan phono.Message
	}
}

// Param provides a way to set parameters to pipe
// returns phono.ParamFunc, which can be executed later
type Param func(p *Pipe) phono.ParamFunc

// stateFn for pipe stateFn machine
type stateFn func(p *Pipe) stateFn

type eventType int

const (
	run eventType = iota
	pause
	resume
)

type event struct {
	eventType
	done chan error
}

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment
	ErrInvalidState = errors.New("Invalid state")
)

func do(action chan struct{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrInvalidState
		}
	}()
	close(action)
	return
}

// New creates a new pipe and applies provided params
func New(params ...Param) (*Pipe, error) {
	p := &Pipe{
		processors: make([]Processor, 0),
		sinks:      make([]Sink, 0),
		paramc:     make(chan *phono.Params),
		eventc:     make(chan event),
	}
	// start state machine
	p.Signal.Ready = make(chan error)
	state := stateFn(ready)
	go func() {
		for state != nil {
			state = state(p)
		}
		p.stop()
	}()
	<-p.Signal.Ready
	for _, param := range params {
		param(p)()
	}
	return p, nil
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
	runEvent := event{
		eventType: run,
		done:      make(chan error),
	}
	p.eventc <- runEvent
	return runEvent.done
}

// Pause switches pipe into pausing state
func (p *Pipe) Pause() chan error {
	pauseEvent := event{
		eventType: pause,
		done:      make(chan error),
	}
	p.eventc <- pauseEvent
	return pauseEvent.done
}

// Resume switches pipe into running state
func (p *Pipe) Resume() chan error {
	resumeEvent := event{
		eventType: resume,
		done:      make(chan error),
	}
	p.eventc <- resumeEvent
	return resumeEvent.done
}

// Validate check's if the pipe is valid and ready to be executed
func (p *Pipe) Validate() error {
	// validate pump
	if p.pump == nil {
		return errors.New("Pump is not defined")
	}
	err := p.pump.Validate()
	if err != nil {
		return err
	}

	// validate sinks
	if p.sinks == nil || len(p.sinks) == 0 {
		return errors.New("Sinks are not defined")
	}
	for _, sink := range p.sinks {
		err := sink.Validate()
		if err != nil {
			return err
		}
	}

	// validate processors
	for _, proc := range p.processors {
		err := proc.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// Wait waits till the Pump is finished
func Wait(errc <-chan error) error {
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

func (p *Pipe) broadcastToSinks(ctx context.Context, in <-chan phono.Message) ([]<-chan error, error) {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan phono.Message, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan phono.Message)
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
	p.message.take = make(chan phono.Message)
	var do struct{}
	return func() (message phono.Message) {
		p.message.ask <- do
		return <-p.message.take
	}
}

// ready state defines that pipe can:
// 	run - start processing
func ready(p *Pipe) stateFn {
	fmt.Println("pipe is ready")
	close(p.Signal.Ready)
	for {
		select {
		case newParams, ok := <-p.paramc:
			if !ok {
				return nil
			}
			newParams.ApplyTo(p)
			p.cachedParams = *p.cachedParams.Join(newParams)
		case event := <-p.eventc:
			fmt.Printf("got new event: %v\n", event)
			switch event.eventType {
			case run:
				// p.run = nil
				ctx, cancelFn := context.WithCancel(context.Background())
				p.cancelFn = cancelFn
				if err := p.Validate(); err != nil {
					event.done <- err
					p.cancelFn()
					return ready
				}
				errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))

				// start pump
				out, errc, err := p.pump.Pump()(ctx, p.soure())
				if err != nil {
					event.done <- err
					p.cancelFn()
					return ready
				}
				errcList = append(errcList, errc)

				// start chained processesing
				for _, proc := range p.processors {
					out, errc, err = proc.Process()(ctx, out)
					if err != nil {
						event.done <- err
						p.cancelFn()
						return ready
					}
					errcList = append(errcList, errc)
				}

				sinkErrcList, err := p.broadcastToSinks(ctx, out)
				if err != nil {
					event.done <- err
					p.cancelFn()
					return ready
				}
				errcList = append(errcList, sinkErrcList...)
				p.errorc = mergeErrors(errcList...)
				p.Signal.Running = make(chan error)
				close(event.done)
				return running
			default:
				event.done <- ErrInvalidState
			}
		}
	}
}

// running state defines that pipe can be:
//	paused - pause the processing
func running(p *Pipe) stateFn {
	fmt.Println("pipe is running")
	p.Signal.Ready = make(chan error)
	close(p.Signal.Running)
	for {
		select {
		case newParams, ok := <-p.paramc:
			if !ok {
				return nil
			}
			newParams.ApplyTo(p)
			p.cachedParams = *p.cachedParams.Join(newParams)
		case <-p.message.ask:
			message := phono.Message{}
			if !p.cachedParams.Empty() {
				message.Params = &p.cachedParams
				p.cachedParams = phono.Params{}
			}
			p.message.take <- message
		case err := <-p.errorc:
			if err != nil {
				p.Signal.Ready <- err
				p.cancelFn()
			}
			return ready
		case event := <-p.eventc:
			fmt.Printf("got new event: %v\n", event)
			switch event.eventType {
			case pause:
				p.Signal.Paused = make(chan error)
				close(event.done)
				return pausing
			default:
				event.done <- ErrInvalidState
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
			message := phono.Message{}
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
				p.Signal.Ready <- err
				p.cancelFn()
			}
			return ready
		}
	}
}

func paused(p *Pipe) stateFn {
	fmt.Println("pipe is paused")
	close(p.Signal.Paused)
	for {
		select {
		case newParams, ok := <-p.paramc:
			if !ok {
				return nil
			}
			newParams.ApplyTo(p)
			p.cachedParams = *p.cachedParams.Join(newParams)
		case event := <-p.eventc:
			fmt.Printf("got new event: %v\n", event)
			switch event.eventType {
			case resume:
				p.Signal.Running = make(chan error)
				close(event.done)
				return running
			default:
				event.done <- ErrInvalidState
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
