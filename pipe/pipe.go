package pipe

import (
	"context"
	"errors"
	"sync"

	"github.com/dudk/phono"
)

// Pump is a source of samples
type Pump interface {
	Pump(phono.Session) phono.PumpFunc
}

// Processor defines interface for pipe-prcessors
type Processor interface {
	Process(phono.Session) phono.ProcessFunc
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(phono.Session) phono.SinkFunc
}

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	session    phono.Session
	pump       Pump
	processors []Processor
	sinks      []Sink
}

// Option provides a way to set options to pipe
// returns previous option value
type Option func(p *Pipe) Option

// New creates a new pipe and applies provided options
func New(session phono.Session, options ...Option) *Pipe {
	pipe := &Pipe{
		session:    session,
		processors: make([]Processor, 0),
		sinks:      make([]Sink, 0),
	}
	for _, option := range options {
		option(pipe)
	}
	return pipe
}

// WithPump sets pump to Pipe
func WithPump(pump Pump) Option {
	return func(p *Pipe) Option {
		previous := p.pump
		p.pump = pump
		return WithPump(previous)
	}
}

// WithProcessors sets processors to Pipe
func WithProcessors(processors ...Processor) Option {
	return func(p *Pipe) Option {
		previous := p.processors
		p.processors = append(p.processors, processors...)
		return WithProcessors(previous...)
	}
}

// WithSinks sets sinks to Pipe
func WithSinks(sinks ...Sink) Option {
	return func(p *Pipe) Option {
		previous := p.sinks
		p.sinks = append(p.sinks, sinks...)
		return WithSinks(previous...)
	}
}

// Validate check's if the pipe is valid and ready to be executed
func (p *Pipe) Validate() error {
	if p.pump == nil {
		return errors.New("Pump is not defined")
	}

	if p.sinks == nil || len(p.sinks) == 0 {
		return errors.New("Sinks are not defined")
	}

	return nil
}

// Run invokes a pipe
func (p *Pipe) Run(ctx context.Context) error {
	if err := p.Validate(); err != nil {
		return err
	}

	errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))
	//start pump
	out, errc, err := p.pump.Pump(p.session)(ctx)
	if err != nil {
		return err
	}
	errcList = append(errcList, errc)

	//start chained processes
	for _, proc := range p.processors {
		out, errc, err = proc.Process(p.session)(ctx, out)
		if err != nil {
			return err
		}
		errcList = append(errcList, errc)
	}

	sinkErrcList, err := p.broadcastToSinks(ctx, out)
	if err != nil {
		return err
	}
	errcList = append(errcList, sinkErrcList...)

	return waitPipe(errcList...)
}

func waitPipe(errcList ...<-chan error) error {
	errc := mergeErrors(errcList...)
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}

// merge error channels
func mergeErrors(errcList ...<-chan error) <-chan error {
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
		errc, err := s.Sink(p.session)(ctx, broadcasts[i])
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
