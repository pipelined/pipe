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
		SampleRate signal.Frequency
		Channels   int
	}

	// Routing defines sequence of DSP components allocators. It has a
	// single source, zero or many processors and single sink.
	Routing struct {
		Source     SourceAllocatorFunc
		Processors []ProcessorAllocatorFunc
		Sink       SinkAllocatorFunc
	}

	// SourceAllocatorFunc returns source for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures.
	SourceAllocatorFunc func(bufferSize int) (Source, error)

	// ProcessorAllocatorFunc returns processor for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures. Along with the processor, output signal properties are
	// returned.
	ProcessorAllocatorFunc func(bufferSize int, output SignalProperties) (Processor, error)

	// SinkAllocatorFunc returns sink for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures.
	SinkAllocatorFunc func(bufferSize int, output SignalProperties) (Sink, error)
)

type (
	// Pipe is a graph formed with multiple lines of bound DSP components.
	Pipe struct {
		bufferSize int
		lines      []Line
	}

	// Line bounds the routing to the context and buffer size.
	Line struct {
		Source
		Processors []Processor
		Sink
	}
	// Source is a source of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Source struct {
		mutability.Mutability
		Output SignalProperties
		SourceFunc
		StartFunc
		FlushFunc
	}

	// SourceFunc takes the output buffer and fills it with a signal data.
	// If no data is available, io.EOF should be returned.
	SourceFunc func(out signal.Floating) (int, error)

	// Processor is a mutator of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Processor struct {
		mutability.Mutability
		Output SignalProperties
		ProcessFunc
		StartFunc
		FlushFunc
	}

	// ProcessFunc takes the input buffer, applies processing logic and writes
	// the result into output buffer.
	ProcessFunc func(in, out signal.Floating) error

	// Sink is a destination of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Sink struct {
		mutability.Mutability
		Output SignalProperties
		SinkFunc
		StartFunc
		FlushFunc
	}

	// SinkFunc takes the input buffer and writes that to the underlying destination.
	SinkFunc func(in signal.Floating) error

	// StartFunc provides a hook to flush all buffers for the component.
	StartFunc func(ctx context.Context) error
	// FlushFunc provides a hook to flush all buffers for the component or
	// execute any other form of finalization logic.
	FlushFunc func(ctx context.Context) error
)

type (
	// Runner executes the pipe.
	Runner struct {
		ctx           context.Context
		cancelFn      context.CancelFunc
		mutability    mutability.Mutability
		bufferSize    int
		merger        *merger
		listeners     map[mutability.Mutability]chan mutability.Mutations
		mutations     map[chan mutability.Mutations]mutability.Mutations
		mutationsChan chan []mutability.Mutation
		errorChan     chan error
		runners       []runner.Runner
	}
)

// New returns a new Pipe that binds multiple lines using the same context
// and buffer size.
func New(bufferSize int, routes ...Routing) (*Pipe, error) {
	if len(routes) == 0 {
		panic("pipe without lines")
	}
	// context for pipe binding.
	// will be cancelled if any binding failes.
	lines := make([]Line, 0, len(routes))
	for i := range routes {
		l, err := routes[i].line(bufferSize)
		if err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}

	return &Pipe{
		bufferSize: bufferSize,
		lines:      lines,
	}, nil
}

// AddRoute adds the route to already bound pipe.
func (p *Pipe) AddRoute(r Routing) (Line, error) {
	// For every added line new child context is created. It allows to
	// cancel it without cancelling parent context of already bound
	// components. If pipe is bound successfully, context is not cancelled.
	l, err := r.line(p.bufferSize)
	if err != nil {
		return Line{}, err
	}
	p.lines = append(p.lines, l)
	return l, nil
}

func (r Routing) line(bufferSize int) (Line, error) {
	source, err := r.Source(bufferSize)
	if err != nil {
		return Line{}, fmt.Errorf("source: %w", err)
	}

	input := source.Output
	processors := make([]Processor, 0, len(r.Processors))
	for i := range r.Processors {
		processor, err := r.Processors[i](bufferSize, input)
		if err != nil {
			return Line{}, fmt.Errorf("processor: %w", err)
		}
		processors = append(processors, processor)
		input = processor.Output
	}

	sink, err := r.Sink(bufferSize, input)
	if err != nil {
		return Line{}, fmt.Errorf("sink: %w", err)
	}

	return Line{
		Source:     source,
		Processors: processors,
		Sink:       sink,
	}, nil
}

// Line binds components. All allocators are executed and wrapped into
// runners. If any of allocators failed, the error will be returned and
// flush hooks won't be triggered.
func (l Line) runner(ctx context.Context, bufferSize int) runner.Runner {
	source, input := l.Source.runner(ctx, bufferSize)

	processors := make([]runner.Processor, 0, len(l.Processors))
	var processor runner.Processor
	for i := range l.Processors {
		processor, input = l.Processors[i].runner(ctx, bufferSize, input)
		processors = append(processors, processor)
	}

	sink := l.Sink.runner(ctx, bufferSize, input)
	return runner.Runner{
		Source:     source,
		Processors: processors,
		Sink:       sink,
	}
}

func (s Source) runner(ctx context.Context, bufferSize int) (runner.Source, SignalProperties) {
	return runner.Source{
		Mutations:  make(chan mutability.Mutations, 1),
		Mutability: s.Mutability,
		OutPool:    signal.GetPoolAllocator(s.Output.Channels, bufferSize, bufferSize),
		Fn:         s.SourceFunc,
		Start:      runner.HookFunc(s.StartFunc),
		Flush:      runner.HookFunc(s.FlushFunc),
	}, s.Output
}

func (p Processor) runner(ctx context.Context, bufferSize int, input SignalProperties) (runner.Processor, SignalProperties) {
	return runner.Processor{
		Mutability: p.Mutability,
		InPool:     signal.GetPoolAllocator(input.Channels, bufferSize, bufferSize),
		OutPool:    signal.GetPoolAllocator(p.Output.Channels, bufferSize, bufferSize),
		Fn:         p.ProcessFunc,
		Flush:      runner.HookFunc(p.FlushFunc),
	}, p.Output
}

func (s Sink) runner(ctx context.Context, bufferSize int, input SignalProperties) runner.Sink {
	return runner.Sink{
		Mutability: s.Mutability,
		InPool:     signal.GetPoolAllocator(input.Channels, bufferSize, bufferSize),
		Fn:         s.SinkFunc,
		Flush:      runner.HookFunc(s.FlushFunc),
	}
}

// Run creates and starts new pipe.
func (p *Pipe) Run(ctx context.Context, initializers ...mutability.Mutation) *Runner {
	ctx, cancelFn := context.WithCancel(ctx)
	runners := make([]runner.Runner, 0, len(p.lines))
	listeners := make(map[mutability.Mutability]chan mutability.Mutations)
	for i := range p.lines {
		r := p.lines[i].runner(ctx, p.bufferSize)
		runners = append(runners, r)
		addListeners(listeners, r)
	}
	mutations := make(map[chan mutability.Mutations]mutability.Mutations)
	for i := range initializers {
		if c := listeners[initializers[i].Mutability]; c != nil {
			mutations[c] = mutations[c].Put(initializers[i])
		}
	}
	// push cached mutators at the start
	push(mutations)
	// start the pipe execution with new context
	// cancel is required to stop the pipe in case of error
	errcs := start(ctx, runners)
	merger := merger{
		errorChan: make(chan error, 1),
	}
	merger.merge(errcs...)
	go merger.wait()

	errc := make(chan error, 1)
	mutationsChan := make(chan []mutability.Mutation, 1)
	runnerMutability := mutability.Mutable()
	go func() {
		defer close(errc)
		for {
			select {
			case ms := <-mutationsChan:
				for _, m := range ms {
					// mutate pipe itself
					if m.Mutability == runnerMutability {
						if err := m.Apply(); err != nil {
							cancelFn()
							merger.await()
							errc <- err
						}
					} else {
						for i := range ms {
							if c := listeners[m.Mutability]; c != nil {
								mutations[c] = mutations[c].Put(ms[i])
							}
						}
						push(mutations)
					}
				}
			case err, ok := <-merger.errorChan:
				// merger has buffer of one error,
				// if more errors happen, they will be ignored.
				if ok {
					cancelFn()
					merger.await()
					errc <- err
				}
				return
			}
		}
	}()
	return &Runner{
		bufferSize:    p.bufferSize,
		ctx:           ctx,
		cancelFn:      cancelFn,
		mutability:    runnerMutability,
		mutations:     mutations,
		mutationsChan: mutationsChan,
		errorChan:     errc,
		merger:        &merger,
		listeners:     listeners,
		runners:       runners,
	}
}

func push(mutations map[chan mutability.Mutations]mutability.Mutations) {
	for c, m := range mutations {
		c <- m
		delete(mutations, c)
	}
}

// TODO: merge all errors
// TODO: distinguish context timeout error
func (m *merger) await() {
	// wait until all groutines stop.
	for {
		// only the first error is propagated.
		if _, ok := <-m.errorChan; !ok {
			break
		}
	}
}

// start starts the execution of pipe.
func start(ctx context.Context, runners []runner.Runner) []<-chan error {
	// start all runners
	// error channel for each component
	errChans := make([]<-chan error, 0, 2*len(runners))
	for i := range runners {
		errChans = append(errChans, runners[i].Run(ctx)...)
	}
	return errChans
}

// Push new mutators into pipe.
// Calling this method after pipe is done will cause a panic.
func (r *Runner) Push(mutations ...mutability.Mutation) {
	r.mutationsChan <- mutations
}

// AddLine adds the line to the pipe.
func (r *Runner) AddLine(l Line) mutability.Mutation {
	return r.mutability.Mutate(func() error {
		runner := l.runner(r.ctx, r.bufferSize)
		addListeners(r.listeners, runner)
		r.merger.merge(runner.Run(r.ctx)...)
		return nil
	})
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorAllocatorFunc) []ProcessorAllocatorFunc {
	return processors
}

// Wait for state transition or first error to occur.
func (r *Runner) Wait() error {
	for err := range r.errorChan {
		if err != nil {
			return err
		}
	}
	return nil
}

func addListeners(listeners map[mutability.Mutability]chan mutability.Mutations, r runner.Runner) {
	listeners[r.Source.Mutability] = r.Mutations
	for i := range r.Processors {
		listeners[r.Processors[i].Mutability] = r.Mutations
	}
	listeners[r.Sink.Mutability] = r.Mutations
}
