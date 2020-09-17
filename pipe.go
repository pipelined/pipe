package pipe

import (
	"context"
	"fmt"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/mutability"
)

type (
	// SignalProperties contains information about input/output signal.
	SignalProperties struct {
		signal.SampleRate
		Channels int
	}
	// SourceAllocatorFunc returns source for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures.
	SourceAllocatorFunc func(int) (Source, SignalProperties, error)

	// ProcessorAllocatorFunc returns processor for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures. Along with the processor, output signal properties are
	// returned.
	ProcessorAllocatorFunc func(int, SignalProperties) (Processor, SignalProperties, error)

	// SinkAllocatorFunc returns sink for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures.
	SinkAllocatorFunc func(int, SignalProperties) (Sink, error)

	// Source is a source of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Source struct {
		mutability.Mutability
		SourceFunc
		FlushFunc
	}

	// Processor is a mutator of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Processor struct {
		mutability.Mutability
		ProcessFunc
		FlushFunc
	}

	// Sink is a destination of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Sink struct {
		mutability.Mutability
		SinkFunc
		FlushFunc
	}

	// SourceFunc takes the output buffer and fills it with a signal data.
	// If no data is available, io.EOF should be returned.
	SourceFunc func(out signal.Floating) (int, error)

	// ProcessFunc takes the input buffer, applies processing logic and writes
	// the result into output buffer.
	ProcessFunc func(in, out signal.Floating) error

	// SinkFunc takes the input buffer and writes that to the underlying destination.
	SinkFunc func(in signal.Floating) error

	// FlushFunc provides a hook to flush all buffers for the component.
	FlushFunc func(context.Context) error
)

type (
	// Routing defines sequence of DSP components allocators. It has a
	// single source, zero or many processors and single sink.
	Routing struct {
		Source     SourceAllocatorFunc
		Processors []ProcessorAllocatorFunc
		Sink       SinkAllocatorFunc
	}

	// Line is a sequence of bound DSP components.
	Line struct {
		numChannels int
		mutators    chan mutability.Mutations
		source      runner.Source
		processors  []runner.Processor
		sink        runner.Sink
	}

	// Pipe listeners the execution of multiple chained lines. Lines might be chained
	// through components, mixer for example. If lines are not chained, they must be
	// controlled by separate Pipes.
	Pipe struct {
		mutability mutability.Mutability
		ctx        context.Context
		cancelFn   context.CancelFunc
		merger     *merger
		lines      []Line
		listeners  map[mutability.Mutability]chan mutability.Mutations
		mutations  map[chan mutability.Mutations]mutability.Mutations
		push       chan []mutability.Mutation
		errors     chan error
	}
)

// Lines is a helper function that allows to bind multiple routes with
// using same buffer size.
func Lines(bufferSize int, routes ...Routing) ([]Line, error) {
	var lines []Line
	for i := range routes {
		l, err := routes[i].Line(bufferSize)
		if err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, nil
}

// Line binds components. All allocators are executed and wrapped into
// runners. If any of allocators failed, the error will be returned and
// flush hooks won't be triggered.
func (r Routing) Line(bufferSize int) (Line, error) {
	source, input, err := r.Source.runner(bufferSize)
	if err != nil {
		return Line{}, fmt.Errorf("error routing %w", err)
	}

	var (
		processors []runner.Processor
		processor  runner.Processor
	)
	for _, fn := range r.Processors {
		processor, input, err = fn.runner(bufferSize, input)
		if err != nil {
			return Line{}, fmt.Errorf("error routing %w", err)
		}
		processors = append(processors, processor)
	}

	sink, err := r.Sink.runner(bufferSize, input)
	if err != nil {
		return Line{}, fmt.Errorf("error routing: %w", err)
	}

	return Line{
		mutators:   make(chan mutability.Mutations, 1),
		source:     source,
		processors: processors,
		sink:       sink,
	}, nil
}

func (l Line) listeners(listeners map[mutability.Mutability]chan mutability.Mutations) {
	listeners[l.source.Mutability] = l.mutators
	for i := range l.processors {
		listeners[l.processors[i].Mutability] = l.mutators
	}
	listeners[l.sink.Mutability] = l.mutators
}

func (fn SourceAllocatorFunc) runner(bufferSize int) (runner.Source, SignalProperties, error) {
	source, output, err := fn(bufferSize)
	if err != nil {
		return runner.Source{}, SignalProperties{}, fmt.Errorf("source: %w", err)
	}
	return runner.Source{
		Mutability: source.Mutability,
		OutPool:    signal.GetPoolAllocator(output.Channels, bufferSize, bufferSize),
		Fn:         source.SourceFunc,
		Flush:      runner.Flush(source.FlushFunc),
	}, output, nil
}

func (fn ProcessorAllocatorFunc) runner(bufferSize int, input SignalProperties) (runner.Processor, SignalProperties, error) {
	processor, output, err := fn(bufferSize, input)
	if err != nil {
		return runner.Processor{}, SignalProperties{}, fmt.Errorf("processor: %w", err)
	}
	return runner.Processor{
		Mutability: processor.Mutability,
		InPool:     signal.GetPoolAllocator(input.Channels, bufferSize, bufferSize),
		OutPool:    signal.GetPoolAllocator(output.Channels, bufferSize, bufferSize),
		Fn:         processor.ProcessFunc,
		Flush:      runner.Flush(processor.FlushFunc),
	}, output, nil
}

func (fn SinkAllocatorFunc) runner(bufferSize int, input SignalProperties) (runner.Sink, error) {
	sink, err := fn(bufferSize, input)
	if err != nil {
		return runner.Sink{}, fmt.Errorf("sink: %w", err)
	}
	return runner.Sink{
		Mutability: sink.Mutability,
		InPool:     signal.GetPoolAllocator(input.Channels, bufferSize, bufferSize),
		Fn:         sink.SinkFunc,
		Flush:      runner.Flush(sink.FlushFunc),
	}, nil
}

// New creates and starts new pipe.
func New(ctx context.Context, options ...Option) Pipe {
	ctx, cancelFn := context.WithCancel(ctx)
	p := Pipe{
		mutability: mutability.Mutable(),
		merger: &merger{
			errors: make(chan error, 1),
		},
		ctx:       ctx,
		cancelFn:  cancelFn,
		listeners: make(map[mutability.Mutability]chan mutability.Mutations),
		mutations: make(map[chan mutability.Mutations]mutability.Mutations),
		lines:     make([]Line, 0),
		push:      make(chan []mutability.Mutation, 1),
		errors:    make(chan error, 1),
	}
	for _, option := range options {
		option(&p)
	}
	if len(p.lines) == 0 {
		panic("pipe without lines")
	}
	// push cached mutators at the start
	push(p.mutations)
	p.merger.merge(start(p.ctx, p.lines)...)
	go p.merger.wait()
	go func() {
		defer close(p.errors)
		for {
			select {
			case mutations := <-p.push:
				for _, m := range mutations {
					// mutate pipe itself
					if m.Mutability == p.mutability {
						if err := m.Apply(); err != nil {
							p.interrupt(err)
						}
					} else {
						for _, m := range mutations {
							if c := p.listeners[m.Mutability]; c != nil {
								p.mutations[c] = p.mutations[c].Put(m)
							}
						}
						push(p.mutations)
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

func push(mutators map[chan mutability.Mutations]mutability.Mutations) {
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
	for i := range lines {
		errChans = append(errChans, lines[i].start(ctx)...)
	}
	return errChans
}

func (l Line) start(ctx context.Context) []<-chan error {
	errChans := make([]<-chan error, 0, 2+len(l.processors))
	// start source
	out, errs := l.source.Run(ctx, l.mutators)
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
func (p Pipe) Push(mutations ...mutability.Mutation) {
	p.push <- mutations
}

// AddLine adds the line to the pipe.
func (p Pipe) AddLine(l Line) mutability.Mutation {
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
func Processors(processors ...ProcessorAllocatorFunc) []ProcessorAllocatorFunc {
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
