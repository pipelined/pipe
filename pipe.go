package pipe

import (
	"context"
	"fmt"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/pool"
)

type (
	Bus struct {
		signal.SampleRate
		Channels int
	}
	// PumpMaker creates new pump structure for provided buffer size.
	// It pre-allocates all necessary buffers and structures.
	PumpMaker func(int) (Pump, Bus, error)
	// Pump is a source of samples. Pump method accepts a new buffer and
	// fills it with signal data. If no data is available, io.EOF should
	// be returned.
	Pump struct {
		mutable.Mutable
		Pump func(out signal.Floating) (int, error)
		Flush
	}

	// ProcessorMaker creates new pump structure for provided buffer size.
	// It pre-allocates all necessary buffers and structures.
	ProcessorMaker func(int, Bus) (Processor, Bus, error)
	// Processor defines interface for pipe processors. It receives two
	// buffers for input and output signal data. Buffer size could be
	// changed during execution, but only decrease allowed. Number of
	// channels cannot be changed.
	Processor struct {
		mutable.Mutable
		Process func(in, out signal.Floating) error
		Flush
	}

	// SinkMaker creates new pump structure for provided buffer size.
	// It pre-allocates all necessary buffers and structures.
	SinkMaker func(int, Bus) (Sink, error)
	// Sink is an interface for final stage in audio pipeline.
	// This components must not change buffer content. Line can have
	// multiple sinks and this will cause race condition.
	Sink struct {
		mutable.Mutable
		Sink func(in signal.Floating) error
		Flush
	}

	// Flush provides a hook to flush all buffers for the component.
	Flush func(context.Context) error
)

type (
	// Route defines sequence of components closures.
	// It has a single pump, zero or many processors, executed
	// sequentially and one or many sinks executed in parallel.
	Route struct {
		Pump       PumpMaker
		Processors []ProcessorMaker
		Sink       SinkMaker
	}

	// Line is a sequence of DSP components.
	Line struct {
		numChannels int
		mutators    chan mutable.Mutations
		pump        runner.Pump
		processors  []runner.Processor
		sink        runner.Sink
	}

	// Pipe listeners the execution of multiple chained lines. Lines might be chained
	// through components, mixer for example.  If lines are not chained, they must be
	// controlled by separate Pipes. Use New constructor to instantiate new Pipes.
	Pipe struct {
		mutability          mutable.Mutable
		ctx                 context.Context
		cancelFn            context.CancelFunc
		merger              *merger
		lines               []Line
		listeners           map[mutable.Mutable]chan mutable.Mutations
		mutatorsByListeners map[chan mutable.Mutations]mutable.Mutations
		push                chan []mutable.Mutation
		errors              chan error
	}
)

// Line line components. All closures are executed and wrapped into runners.
func (l Route) Line(bufferSize int) (Line, error) {
	pump, input, err := l.Pump.runner(bufferSize)
	if err != nil {
		return Line{}, fmt.Errorf("error routing %w", err)
	}

	var (
		processors []runner.Processor
		processor  runner.Processor
	)
	for _, fn := range l.Processors {
		processor, input, err = fn.runner(bufferSize, input)
		if err != nil {
			return Line{}, fmt.Errorf("error routing %w", err)
		}
		processors = append(processors, processor)
	}

	sink, err := l.Sink.runner(bufferSize, input)
	if err != nil {
		return Line{}, fmt.Errorf("error routing sink: %w", err)
	}

	return Line{
		mutators:   make(chan mutable.Mutations, 1),
		pump:       pump,
		processors: processors,
		sink:       sink,
	}, nil
}

func (l Line) listeners(listeners map[mutable.Mutable]chan mutable.Mutations) {
	listeners[l.pump.Mutability] = l.mutators
	for i := range l.processors {
		listeners[l.processors[i].Mutability] = l.mutators
	}
	listeners[l.sink.Mutability] = l.mutators
}

func (fn PumpMaker) runner(bufferSize int) (runner.Pump, Bus, error) {
	pump, bus, err := fn(bufferSize)
	if err != nil {
		return runner.Pump{}, Bus{}, fmt.Errorf("pump: %w", err)
	}
	return runner.Pump{
		Mutability: pump.Mutable,
		Output: pool.Get(signal.Allocator{
			Channels: bus.Channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		}),
		Fn:    pump.Pump,
		Flush: runner.Flush(pump.Flush),
		Meter: metric.Meter(pump, bus.SampleRate),
	}, bus, nil
}

func (fn ProcessorMaker) runner(bufferSize int, input Bus) (runner.Processor, Bus, error) {
	processor, output, err := fn(bufferSize, input)
	if err != nil {
		return runner.Processor{}, Bus{}, fmt.Errorf("processor: %w", err)
	}
	return runner.Processor{
		Mutability: processor.Mutable,
		Input: pool.Get(signal.Allocator{
			Channels: input.Channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		}),
		Output: pool.Get(signal.Allocator{
			Channels: output.Channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		}),
		Fn:    processor.Process,
		Flush: runner.Flush(processor.Flush),
		Meter: metric.Meter(processor, output.SampleRate),
	}, output, nil
}

func (fn SinkMaker) runner(bufferSize int, input Bus) (runner.Sink, error) {
	sink, err := fn(bufferSize, input)
	if err != nil {
		return runner.Sink{}, fmt.Errorf("sink: %w", err)
	}
	return runner.Sink{
		Mutability: sink.Mutable,
		Input: pool.Get(signal.Allocator{
			Channels: input.Channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		}),
		Fn:    sink.Sink,
		Flush: runner.Flush(sink.Flush),
		Meter: metric.Meter(sink, input.SampleRate),
	}, nil
}

// New creates a new pipeline.
// Returned pipeline is in Ready state.
func New(ctx context.Context, options ...Option) Pipe {
	ctx, cancelFn := context.WithCancel(ctx)
	p := Pipe{
		mutability: mutable.New(),
		merger: &merger{
			errors: make(chan error, 1),
		},
		ctx:                 ctx,
		cancelFn:            cancelFn,
		listeners:           make(map[mutable.Mutable]chan mutable.Mutations),
		mutatorsByListeners: make(map[chan mutable.Mutations]mutable.Mutations),
		lines:               make([]Line, 0),
		push:                make(chan []mutable.Mutation, 1),
		errors:              make(chan error, 1),
	}
	for _, option := range options {
		option(&p)
	}
	if len(p.lines) == 0 {
		panic("pipe without lines")
	}
	// push cached mutators at the start
	push(p.mutatorsByListeners)
	p.merger.merge(start(p.ctx, p.lines)...)
	go p.merger.wait()
	go func() {
		defer close(p.errors)
		for {
			select {
			case mutations := <-p.push:
				for _, m := range mutations {
					// mutate pipe itself
					if m.Mutable == p.mutability {
						if err := m.Apply(); err != nil {
							p.interrupt(err)
						}
					} else {
						for _, m := range mutations {
							if c := p.listeners[m.Mutable]; c != nil {
								p.mutatorsByListeners[c] = p.mutatorsByListeners[c].Put(m)
							}
						}
						push(p.mutatorsByListeners)
					}
				}
			case err, ok := <-p.merger.errors:
				// merger has buffer of one error,
				// if more errors happen, they will be ignored.
				if ok {
					p.interrupt(err)
				}
				return
			}
		}
	}()
	return p
}

func push(mutators map[chan mutable.Mutations]mutable.Mutations) {
	for c, m := range mutators {
		c <- m
	}
}

func (p Pipe) interrupt(err error) {
	p.cancelFn()
	// wait until all groutines stop.
	for {
		// only the first error is propagated.
		if _, ok := <-p.merger.errors; !ok {
			break
		}
	}
	p.errors <- fmt.Errorf("pipe error: %w", err)
}

// start starts the execution of pipe.
func start(ctx context.Context, lines []Line) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, 2*len(lines))
	for _, l := range lines {
		errChans = append(errChans, l.start(ctx)...)
	}
	return errChans
}

func (l Line) start(ctx context.Context) []<-chan error {
	errChans := make([]<-chan error, 0, 2+len(l.processors))
	// start pump
	out, errs := l.pump.Run(ctx, l.mutators)
	errChans = append(errChans, errs)

	// start chained processesing
	for _, proc := range l.processors {
		out, errs = proc.Run(ctx, out)
		errChans = append(errChans, errs)
	}

	errs = l.sink.Run(ctx, out)
	errChans = append(errChans, errs)
	return errChans
}

// Push new mutators into pipe.
// Calling this method after pipe is done will cause a panic.
func (p Pipe) Push(mutations ...mutable.Mutation) {
	p.push <- mutations
}

func (p Pipe) AddLine(l Line) mutable.Mutation {
	return p.mutability.Mutate(func() error {
		addLine(&p, l)
		p.merger.merge(l.start(p.ctx)...)
		return nil
	})
}

func addLine(p *Pipe, l Line) {
	p.lines = append(p.lines, l)
	l.listeners(p.listeners)
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorMaker) []ProcessorMaker {
	return processors
}

// Wait for state transition or first error to occur.
func (p Pipe) Wait() error {
	for err := range p.errors {
		if err != nil {
			return err
		}
	}
	return nil
}
