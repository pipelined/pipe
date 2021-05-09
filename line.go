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

	// line represents bound components
	line struct {
		context    mutable.Context
		source     Source
		processors []Processor
		sink       Sink
	}
)

func (l Line) line(bufferSize int) (*line, error) {
	fitFn := fitting.Async
	if l.Context.IsMutable() {
		fitFn = fitting.Sync
	}
	source, err := l.Source.allocate(componentContext(l.Context), bufferSize)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

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

	processors := make([]Processor, 0, len(l.Processors))
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
		processors = append(processors, processor)
	}

	sink, err := l.Sink.allocate(componentContext(l.Context), bufferSize, link.SignalProperties)
	if err != nil {
		return nil, fmt.Errorf("sink: %w", err)
	}
	sink.in.PoolAllocator = link.PoolAllocator
	sink.in.Receiver = link.Fitting

	return &line{
		context:    l.Context,
		source:     source,
		processors: processors,
		sink:       sink,
	}, nil
}

func (l *line) bindExecutors(executors map[mutable.Context]executor, p mutable.Pusher) {
	if l.context.IsMutable() {
		if e, ok := executors[l.context]; ok {
			mle := e.(*multiLineExecutor)
			mle.Lines = append(mle.Lines, l.executor(mle.Destination))
		} else {
			d := mutable.NewDestination()
			p.AddDestination(l.context, d)
			executors[l.context] = &multiLineExecutor{
				Context:     l.context,
				Destination: d,
				Lines:       []*lineExecutor{l.executor(d)},
			}
		}
		return
	}

	d := mutable.NewDestination()
	l.source.dest = d
	p.AddDestination(l.source.Context, d)
	executors[l.source.Context] = l.source
	for _, proc := range l.processors {
		p.AddDestination(proc.Context, d)
		executors[proc.Context] = proc
	}
	p.AddDestination(l.sink.Context, d)
	executors[l.sink.Context] = l.sink
}

func (l line) executor(d mutable.Destination) *lineExecutor {
	executors := make([]executor, 0, 2+len(l.processors))
	l.source.dest = d
	executors = append(executors, l.source)
	for _, p := range l.processors {
		executors = append(executors, p)
	}
	executors = append(executors, l.sink)
	return &lineExecutor{
		executors: executors,
	}
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
