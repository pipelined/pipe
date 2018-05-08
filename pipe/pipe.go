package pipe

import (
	"context"
	"errors"
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
	phono.OptionUser
	Process() phono.ProcessFunc
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	phono.OptionUser
	Sink() phono.SinkFunc
}

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	ctx        context.Context
	cancelFn   context.CancelFunc
	pump       Pump
	processors []Processor
	sinks      []Sink

	cachedOptions phono.Options
	options       chan *phono.Options
	sync.Mutex
	run    action
	pause  action
	resume action
	init   chan struct{}
	// closed when run finshed correctly
	interrupt chan error
	// closed when pipe.interrupt is closed
	Interrupt chan error
	message   struct {
		ask  chan struct{}
		take chan phono.Message
	}
	actionCallback func()
}

// Option provides a way to set options to pipe
// returns phono.Option function, which can be executed later
type Option func(p *Pipe) phono.OptionFunc

// state for pipe state machine
type state func(p *Pipe) state

// action performed by user of state machine
type action struct {
	start     chan struct{}
	interrupt chan error
}

var (
	// ErrInvalidState is returned if pipe method cannot be executed at this moment
	ErrInvalidState = errors.New("Invalid state")
	do              struct{}
)

func (a *action) do() (done chan error, err error) {
	done = a.interrupt
	defer func() {
		if r := recover(); r != nil {
			done = nil
			err = ErrInvalidState
		}
	}()
	close(a.start)
	return
}

func (p *Pipe) handle(a action) {
	a.start = nil
	p.actionCallback = a.done
}

func (a *action) done() {
	close(a.interrupt)
	a.interrupt = nil
}

func (a *action) initiate() {
	a.start = make(chan struct{})
	a.interrupt = make(chan error)
}

// New creates a new pipe and applies provided options
func New(options ...Option) (*Pipe, error) {
	p := &Pipe{
		processors: make([]Processor, 0),
		sinks:      make([]Sink, 0),
		options:    make(chan *phono.Options),
	}
	p.ctx, p.cancelFn = context.WithCancel(context.Background())
	// start state machine
	p.init = make(chan struct{})
	state := state(ready)
	p.Lock()
	go func() {
		for state != nil {
			state = state(p)
		}
		p.stop()
	}()
	<-p.init
	p.init = nil
	for _, option := range options {
		option(p)()
	}
	return p, nil
}

// WithPump sets pump to Pipe
func WithPump(pump Pump) Option {
	return func(p *Pipe) phono.OptionFunc {
		return func() {
			p.pump = pump
		}
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...Processor) Option {
	return func(p *Pipe) phono.OptionFunc {
		return func() {
			p.processors = processors
		}
	}
}

// WithSinks sets sinks to Pipe
func WithSinks(sinks ...Sink) Option {
	return func(p *Pipe) phono.OptionFunc {
		return func() {
			p.sinks = sinks
		}
	}
}

// Run switches pipe into running state
func (p *Pipe) Run() (chan error, error) {
	p.Lock()
	defer p.Unlock()
	return p.run.do()
}

// Pause switches pipe into pausing state
func (p *Pipe) Pause() (chan error, error) {
	p.Lock()
	defer p.Unlock()
	return p.pause.do()
}

// Resume switches pipe into running state
func (p *Pipe) Resume() (chan error, error) {
	p.Lock()
	defer p.Unlock()
	return p.resume.do()
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

func (p *Pipe) broadcastToSinks(in <-chan phono.Message) ([]<-chan error, error) {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan phono.Message, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan phono.Message)
	}

	//start broadcast
	for i, s := range p.sinks {
		errc, err := s.Sink()(p.ctx, broadcasts[i])
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
			case <-p.ctx.Done():
				return
			}
		}
	}()

	return errcList, nil
}

// soure returns a default message producer which caches options
// if new options are pushed into pipe - next message will contain them
// if pipe paused this call will block
func (p *Pipe) soure() phono.NewMessageFunc {
	var do struct{}
	return func() (message phono.Message) {
		p.message.ask <- do
		return <-p.message.take
	}
}

// ready state defines that pipe can:
// 	run - start processing
func ready(p *Pipe) state {
	p.message.ask = make(chan struct{})
	p.message.take = make(chan phono.Message)
	p.run.initiate()
	p.Unlock()
	defer p.Lock()
	for {
		select {
		case p.init <- do:
		case newOptions, ok := <-p.options:
			if !ok {
				return nil
			}
			newOptions.ApplyTo(p)
			p.cachedOptions = *p.cachedOptions.Join(newOptions)
		case <-p.run.start:
			p.handle(p.run)
			p.Interrupt = make(chan error)
			if err := p.Validate(); err != nil {
				p.run.interrupt <- err
			}
			errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))

			// start pump
			out, errc, err := p.pump.Pump()(p.ctx, p.soure())
			if err != nil {
				p.run.interrupt <- err
			}
			errcList = append(errcList, errc)

			// start chained processesing
			for _, proc := range p.processors {
				out, errc, err = proc.Process()(p.ctx, out)
				if err != nil {
					p.run.interrupt <- err
				}
				errcList = append(errcList, errc)
			}

			sinkErrcList, err := p.broadcastToSinks(out)
			if err != nil {
				p.run.interrupt <- err
			}
			errcList = append(errcList, sinkErrcList...)

			p.interrupt = mergeErrors(errcList...)
			return running
		}
	}
}

// running state defines that pipe can be:
//	paused - pause the processing
func running(p *Pipe) state {
	p.pause.initiate()
	p.actionCallback()
	p.Unlock()
	defer p.Lock()
	for {
		select {
		case <-p.pause.start:
			p.handle(p.pause)
			return pausing
		case newOptions, ok := <-p.options:
			if !ok {
				return nil
			}
			newOptions.ApplyTo(p)
			p.cachedOptions = *p.cachedOptions.Join(newOptions)
		case <-p.message.ask:
			message := phono.Message{}
			if !p.cachedOptions.Empty() {
				message.Options = &p.cachedOptions
				p.cachedOptions = phono.Options{}
			}
			p.message.take <- message
		case err, failed := <-p.interrupt:
			if !failed {
				close(p.Interrupt)
			} else {
				p.Interrupt <- err
			}
			p.actionCallback = nil
			return ready
		}
	}
}

// pausing defines a state when pipe accepted a pause command and pushed message with confirmation
func pausing(p *Pipe) state {
	p.Unlock()
	defer p.Lock()
	for {
		select {
		case newOptions, ok := <-p.options:
			if !ok {
				return nil
			}
			newOptions.ApplyTo(p)
			p.cachedOptions = *p.cachedOptions.Join(newOptions)
		case <-p.message.ask:
			message := phono.Message{}
			if !p.cachedOptions.Empty() {
				message.Options = &p.cachedOptions
				p.cachedOptions = phono.Options{}
			}
			message.WaitGroup = &sync.WaitGroup{}
			message.Add(len(p.sinks))
			p.message.take <- message
			message.Wait()
			return paused
		case err, failed := <-p.interrupt:
			if !failed {
				close(p.Interrupt)
			} else {
				p.Interrupt <- err
			}
			p.actionCallback = nil
			return ready
		}
	}
}

func paused(p *Pipe) state {
	p.resume.initiate()
	p.actionCallback()
	p.Unlock()
	defer p.Lock()
	for {
		select {
		case newOptions, ok := <-p.options:
			if !ok {
				return nil
			}
			newOptions.ApplyTo(p)
			p.cachedOptions = *p.cachedOptions.Join(newOptions)
		case <-p.resume.start:
			p.handle(p.resume)
			return running
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

// Push new options into pipe
func (p *Pipe) Push(o *phono.Options) {
	p.options <- o
}

// Close must be called to clean up pipe's resources
func (p *Pipe) Close() {
	p.Lock()
	defer p.Unlock()
	close(p.options)
}
