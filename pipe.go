package pipe

import (
	"context"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/fitting"
	"pipelined.dev/pipe/internal/run"
	"pipelined.dev/pipe/mutable"
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
	SourceAllocatorFunc func(mctx mutable.Context, bufferSize int) (Source, error)

	// ProcessorAllocatorFunc returns processor for provided buffer size.
	// It is responsible for pre-allocation of all necessary buffers and
	// structures. Along with the processor, output signal properties are
	// returned.
	ProcessorAllocatorFunc func(mctx mutable.Context, bufferSize int, input SignalProperties) (Processor, error)

	// SinkAllocatorFunc returns sink for provided buffer size. It is
	// responsible for pre-allocation of all necessary buffers and
	// structures.
	SinkAllocatorFunc func(mctx mutable.Context, bufferSize int, input SignalProperties) (Sink, error)
)

type (
	// Pipe is a graph formed with multiple lines of bound DSP components.
	Pipe struct {
		bufferSize int
		executors  map[chan mutable.Mutations]executor
		contexts   map[mutable.Context]chan mutable.Mutations
	}

	// Source is a source of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Source struct {
		mutable.Context
		SourceFunc
		StartFunc
		FlushFunc
		SignalProperties
		out       connector
		mutations chan mutable.Mutations
	}

	// SourceFunc takes the output buffer and fills it with a signal data.
	// If no data is available, io.EOF should be returned.
	SourceFunc func(out signal.Floating) (int, error)

	// Processor is a mutator of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Processor struct {
		mutable.Context
		ProcessFunc
		StartFunc
		FlushFunc
		SignalProperties
		in  connector
		out connector
	}

	// ProcessFunc takes the input buffer, applies processing logic and
	// writes the result into output buffer.
	ProcessFunc func(in, out signal.Floating) (int, error)

	// Sink is a destination of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Sink struct {
		mutable.Context
		SinkFunc
		StartFunc
		FlushFunc
		SignalProperties
		in connector
	}

	// SinkFunc takes the input buffer and writes that to the underlying
	// destination.
	SinkFunc func(in signal.Floating) error

	// StartFunc provides a hook to flush all buffers for the component.
	StartFunc func(ctx context.Context) error
	// FlushFunc provides a hook to flush all buffers for the component or
	// execute any other form of finalization logic.
	FlushFunc func(ctx context.Context) error

	connector struct {
		fitting.Fitting
		*signal.PoolAllocator
	}
)

// New returns a new Pipe that binds multiple lines using the provided
// buffer size.
func New(bufferSize int, routes ...Routing) (*Pipe, error) {
	if len(routes) == 0 {
		panic("pipe without lines")
	}
	executors := make(map[chan mutable.Mutations]executor)
	contexts := make(map[mutable.Context]chan mutable.Mutations)
	for _, r := range routes {
		if !r.Context.IsMutable() { // async execution
			mutations := make(chan mutable.Mutations, 1)
			executor, ctx, err := r.executor(mutations, bufferSize, fitting.Async)
			if err != nil {
				return nil, err
			}
			executors[mutations] = executor
			contexts[ctx] = mutations
			continue
		}

		// sync exec
		if mutations, ok := contexts[r.Context]; !ok {
			mutations = make(chan mutable.Mutations, 1)
			ex, ctx, err := r.executor(mutations, bufferSize, fitting.Async)
			if err != nil {
				return nil, err
			}
			executors[mutations] = executor{
				Executors: []run.Executor{ex},
			}
			contexts[ctx] = mutations
		} else {
			newEx, _, err := r.executor(mutations, bufferSize, fitting.Async)
			if err != nil {
				return nil, err
			}
			ex := executors[mutations]
			ex.Executors = append(ex.Executors, newEx)
			executors[mutations] = ex
		}
	}

	return &Pipe{
		bufferSize: bufferSize,
		executors:  executors,
	}, nil
}

// AddLine creates the line for provied route and adds it to the pipe.
// func (p *Pipe) AddLine(r Routing) (Line, error) {
// 	l, err := r.line(nil, p.bufferSize)
// 	if err != nil {
// 		return Line{}, err
// 	}
// 	p.lines = append(p.Lines, l)
// 	return l, nil
// }

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorAllocatorFunc) []ProcessorAllocatorFunc {
	return processors
}

// Execute does a single iteration of source component. io.EOF is returned
// if context is done.
func (s Source) Execute(ctx context.Context) error {
	var ms mutable.Mutations
	select {
	case ms = <-s.mutations:
		if err := ms.ApplyTo(s.Context); err != nil {
			return err
		}
	case <-ctx.Done():
		s.out.Close()
		return io.EOF
	default:
	}

	output := s.out.GetFloat64()
	var (
		read int
		err  error
	)
	if read, err = s.SourceFunc(output); err != nil {
		s.out.Close()
		output.Free(s.out.PoolAllocator)
		return err
	}
	if read != output.Length() {
		output = output.Slice(0, read)
	}

	if !s.out.Send(ctx, fitting.Message{Signal: output, Mutations: ms}) {
		s.out.Close()
		return io.EOF
	}
	return nil
}

// Execute does a single iteration of processor component. io.EOF is
// returned if context is done.
func (p Processor) Execute(ctx context.Context) error {
	m, ok := p.in.Receive(ctx)
	if !ok {
		p.out.Close()
		return io.EOF
	}
	if err := m.Mutations.ApplyTo(p.Context); err != nil {
		return err
	}

	output := p.out.GetFloat64()
	if processed, err := p.ProcessFunc(m.Signal, output); err != nil {
		p.out.Close()
		return err
	} else if processed != p.out.Length {
		output = output.Slice(0, processed)
	}

	if !p.out.Send(ctx, fitting.Message{Signal: output, Mutations: m.Mutations}) {
		p.out.Close()
		output.Free(p.out.PoolAllocator)
		return io.EOF
	}
	m.Signal.Free(p.in.PoolAllocator)
	return nil
}

// Execute does a single iteration of sink component. io.EOF is returned if
// context is done.
func (s Sink) Execute(ctx context.Context) error {
	m, ok := s.in.Receive(ctx)
	if !ok {
		return io.EOF
	}
	if err := m.Mutations.ApplyTo(s.Context); err != nil {
		return err
	}

	err := s.SinkFunc(m.Signal)
	m.Signal.Free(s.in.PoolAllocator)
	return err
}

// Start calls the start hook.
func (fn StartFunc) Start(ctx context.Context) error {
	return callHook(ctx, fn)
}

// Flush calls the flush hook.
func (fn FlushFunc) Flush(ctx context.Context) error {
	return callHook(ctx, fn)
}

func callHook(ctx context.Context, hook func(context.Context) error) error {
	if hook == nil {
		return nil
	}
	return hook(ctx)
}

func (sp SignalProperties) poolAllocator(bufferSize int) *signal.PoolAllocator {
	return signal.GetPoolAllocator(sp.Channels, bufferSize, bufferSize)
}
