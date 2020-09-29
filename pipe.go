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

	// SourceAllocatorFunc returns source for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures.
	SourceAllocatorFunc func(ctx context.Context, bufferSize int) (Source, SignalProperties, error)

	// ProcessorAllocatorFunc returns processor for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures. Along with the processor, output signal properties are
	// returned.
	ProcessorAllocatorFunc func(ctx context.Context, bufferSize int, sp SignalProperties) (Processor, SignalProperties, error)

	// SinkAllocatorFunc returns sink for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures.
	SinkAllocatorFunc func(ctx context.Context, bufferSize int, sp SignalProperties) (Sink, error)

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
	// Line defines sequence of DSP components allocators. It has a
	// single source, zero or many processors and single sink.
	Line struct {
		Source     SourceAllocatorFunc
		Processors []ProcessorAllocatorFunc
		Sink       SinkAllocatorFunc
	}

	// Pipe is a sequence of bound DSP components.
	Pipe struct {
		ctx        context.Context
		cancelFn   context.CancelFunc
		bufferSize int
		runners    []*runner.Runner
	}

	// Runner executes the pipe.
	Runner struct {
		pipe          *Pipe
		mutability    mutability.Mutability
		bufferSize    int
		merger        *merger
		listeners     map[mutability.Mutability]chan mutability.Mutations
		mutations     map[chan mutability.Mutations]mutability.Mutations
		mutationsChan chan []mutability.Mutation
		errorChan     chan error
	}
)

// New returns a new Pipe that binds multiple lines using the same context
// and buffer size.
func New(ctx context.Context, bufferSize int, lines ...*Line) (*Pipe, error) {
	ctx, cancelFn := context.WithCancel(ctx)
	if len(lines) == 0 {
		panic("pipe without lines")
	}
	var runners []*runner.Runner
	for i := range lines {
		r, err := lines[i].runner(ctx, bufferSize)
		if err != nil {
			cancelFn()
			return nil, err
		}
		runners = append(runners, r)
	}

	return &Pipe{
		ctx:        ctx,
		cancelFn:   cancelFn,
		bufferSize: bufferSize,
		runners:    runners,
	}, nil
}

// Line binds components. All allocators are executed and wrapped into
// runners. If any of allocators failed, the error will be returned and
// flush hooks won't be triggered.
func (r Line) runner(ctx context.Context, bufferSize int) (*runner.Runner, error) {
	source, input, err := r.Source.runner(ctx, bufferSize)
	if err != nil {
		return nil, fmt.Errorf("error routing %w", err)
	}

	var (
		processors []runner.Processor
		processor  runner.Processor
	)
	for _, fn := range r.Processors {
		processor, input, err = fn.runner(ctx, bufferSize, input)
		if err != nil {
			return nil, fmt.Errorf("error routing %w", err)
		}
		processors = append(processors, processor)
	}

	sink, err := r.Sink.runner(ctx, bufferSize, input)
	if err != nil {
		return nil, fmt.Errorf("error routing: %w", err)
	}

	return &runner.Runner{
		Mutators:   make(chan mutability.Mutations, 1),
		Source:     source,
		Processors: processors,
		Sink:       sink,
	}, nil
}

func (fn SourceAllocatorFunc) runner(ctx context.Context, bufferSize int) (runner.Source, SignalProperties, error) {
	source, output, err := fn(ctx, bufferSize)
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

func (fn ProcessorAllocatorFunc) runner(ctx context.Context, bufferSize int, input SignalProperties) (runner.Processor, SignalProperties, error) {
	processor, output, err := fn(ctx, bufferSize, input)
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

func (fn SinkAllocatorFunc) runner(ctx context.Context, bufferSize int, input SignalProperties) (runner.Sink, error) {
	sink, err := fn(ctx, bufferSize, input)
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

// Run creates and starts new pipe.
func (p *Pipe) Run(initializers ...mutability.Mutation) *Runner {
	listeners := make(map[mutability.Mutability]chan mutability.Mutations)
	for i := range p.runners {
		addListeners(listeners, p.runners[i])
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
	errcs := start(p.ctx, p.runners)
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
							p.cancelFn()
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
					p.cancelFn()
					merger.await()
					errc <- err
				}
				return
			}
		}
	}()
	return &Runner{
		pipe:          p,
		mutability:    runnerMutability,
		mutations:     mutations,
		mutationsChan: mutationsChan,
		errorChan:     errc,
		merger:        &merger,
		listeners:     listeners,
	}
}

func push(mutators map[chan mutability.Mutations]mutability.Mutations) {
	for c, m := range mutators {
		c <- m
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
func start(ctx context.Context, runners []*runner.Runner) []<-chan error {
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
func (r *Runner) AddLine(l *Line) mutability.Mutation {
	return r.mutability.Mutate(func() error {
		runner, err := l.runner(r.pipe.ctx, r.pipe.bufferSize)
		if err != nil {
			return err
		}
		addLine(r, runner)
		r.merger.merge(runner.Run(r.pipe.ctx)...)
		return nil
	})
}

func addLine(rn *Runner, r *runner.Runner) {
	rn.pipe.runners = append(rn.pipe.runners, r)
	addListeners(rn.listeners, r)
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

func addListeners(listeners map[mutability.Mutability]chan mutability.Mutations, r *runner.Runner) {
	listeners[r.Source.Mutability] = r.Mutators
	for i := range r.Processors {
		listeners[r.Processors[i].Mutability] = r.Mutators
	}
	listeners[r.Sink.Mutability] = r.Mutators
}
