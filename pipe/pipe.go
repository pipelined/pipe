package pipe

import (
	"context"
	"errors"
	"sync"

	"github.com/dudk/phono"

	"github.com/dudk/phono/pipe/processor"
	"github.com/dudk/phono/pipe/pump"
	"github.com/dudk/phono/pipe/sink"
)

//Pipe is a pipeline with fully defined sound processing sequence
//it has:
//	1 		pump
//	0..n 	processors
//	1..n	sinks
type Pipe struct {
	pump       pump.Pump
	processors []processor.Processor
	sinks      []sink.Sink
}

//Option provides a way to set options to pipe
//returns previous option value
type Option func(p *Pipe) Option

//New creates a new pipe and applies provided options
func New(options ...Option) *Pipe {
	pipe := &Pipe{
		processors: make([]processor.Processor, 0),
		sinks:      make([]sink.Sink, 0),
	}
	for _, option := range options {
		option(pipe)
	}
	return pipe
}

//Pump sets pump to Pipe
func Pump(pump pump.Pump) Option {
	return func(p *Pipe) Option {
		previous := p.pump
		p.pump = pump
		return Pump(previous)
	}
}

//Processors sets processors to Pipe
func Processors(processors ...processor.Processor) Option {
	return func(p *Pipe) Option {
		previous := p.processors
		p.processors = append(p.processors, processors...)
		return Processors(previous...)
	}
}

//Sinks sets sinks to Pipe
func Sinks(sinks ...sink.Sink) Option {
	return func(p *Pipe) Option {
		previous := p.sinks
		p.sinks = append(p.sinks, sinks...)
		return Sinks(previous...)
	}
}

//Run invokes a pipe
func (p *Pipe) Run(ctx context.Context) error {
	if p.pump == nil {
		return errors.New("Pump is not defined")
	}

	if p.sinks == nil || len(p.sinks) == 0 {
		return errors.New("Sinks are not defined")
	}
	errcList := make([]<-chan error, 0, 1+len(p.processors)+len(p.sinks))
	//start pump
	out, errc, err := p.pump.Pump(ctx)
	if err != nil {
		return err
	}
	errcList = append(errcList, errc)

	//start chained processes
	for _, proc := range p.processors {
		out, errc, err = proc.Process(ctx, out)
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

//merge error channels
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

func (p *Pipe) broadcastToSinks(ctx context.Context, in <-chan phono.Buffer) ([]<-chan error, error) {
	//init errcList for sinks error channels
	errcList := make([]<-chan error, 0, len(p.sinks))
	//list of channels for broadcast
	broadcasts := make([]chan phono.Buffer, len(p.sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan phono.Buffer)
	}

	//start broadcast
	for i, s := range p.sinks {
		errc, err := s.Sink(ctx, broadcasts[i])
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
