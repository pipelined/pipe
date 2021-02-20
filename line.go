package pipe

import (
	"fmt"

	"pipelined.dev/pipe/internal/fitting"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type (
	// Line defines sequence of DSP components allocators. It has a
	// single source, zero or many processors and single sink.
	Line struct {
		mutable.Context
		Source     SourceAllocatorFunc
		Processors []ProcessorAllocatorFunc
		Sink       SinkAllocatorFunc
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

	// SignalProperties contains information about input/output signal.
	SignalProperties struct {
		SampleRate signal.Frequency
		Channels   int
	}
)

// Runner binds routing components together. Line is a set of components
// ready for execution. If Runner is async then source context is returned.
func (l Line) Runner(bufferSize int, dest mutable.Destination) (*LineRunner, error) {
	fitFn := fitting.Async
	if l.Context.IsMutable() {
		fitFn = fitting.Sync
	}
	executors := make([]executor, 0, 2+len(l.Processors))
	source, err := l.Source.allocate(componentContext(l.Context), bufferSize)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	source.dest = dest

	// link holds properties that links two executors
	link := struct {
		fitting.Fitting
		*signal.PoolAllocator
		SignalProperties
	}{
		Fitting:          fitFn(),
		PoolAllocator:    source.out.PoolAllocator,
		SignalProperties: source.SignalProperties,
	}
	source.out.Sender = link.Fitting
	executors = append(executors, source)

	// processors := make([]Processor, 0, len(r.Processors))
	for i := range l.Processors {
		processor, err := l.Processors[i].allocate(componentContext(l.Context), bufferSize, link.SignalProperties)
		if err != nil {
			return nil, fmt.Errorf("processor: %w", err)
		}
		processor.in.PoolAllocator = link.PoolAllocator
		processor.in.Receiver = link.Fitting
		link.Fitting = fitFn()
		processor.out.Sender = link.Fitting
		link.SignalProperties = processor.SignalProperties
		link.PoolAllocator = processor.out.PoolAllocator
		executors = append(executors, processor)
	}

	sink, err := l.Sink.allocate(componentContext(l.Context), bufferSize, link.SignalProperties)
	if err != nil {
		return nil, fmt.Errorf("sink: %w", err)
	}
	sink.in.PoolAllocator = link.PoolAllocator
	sink.in.Receiver = link.Fitting
	executors = append(executors, sink)

	return &LineRunner{
		context:   source.Context,
		executors: executors,
	}, nil
}

func (fn SourceAllocatorFunc) allocate(ctx mutable.Context, bufferSize int) (Source, error) {
	c, err := fn(ctx, bufferSize)
	if err != nil {
		return Source{}, err
	}
	c.Context = ctx
	c.out = out{
		PoolAllocator: c.SignalProperties.poolAllocator(bufferSize),
	}
	return c, nil
}

func (fn ProcessorAllocatorFunc) allocate(ctx mutable.Context, bufferSize int, input SignalProperties) (Processor, error) {
	c, err := fn(ctx, bufferSize, input)
	if err != nil {
		return Processor{}, err
	}
	c.Context = ctx
	c.out = out{
		PoolAllocator: c.SignalProperties.poolAllocator(bufferSize),
	}
	return c, nil
}

func (fn SinkAllocatorFunc) allocate(ctx mutable.Context, bufferSize int, input SignalProperties) (Sink, error) {
	c, err := fn(ctx, bufferSize, input)
	if err != nil {
		return Sink{}, err
	}
	c.Context = ctx
	return c, nil
}

func componentContext(routeContext mutable.Context) mutable.Context {
	if routeContext.IsMutable() {
		return routeContext
	}
	return mutable.Mutable()
}
