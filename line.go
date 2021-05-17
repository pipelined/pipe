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

	// route represents bound components
	route struct {
		context    mutable.Context
		source     *Source
		processors []*Processor
		sink       *Sink
	}

	out struct {
		sender    fitting.Fitting
		allocator *signal.PoolAllocator
	}

	in struct {
		receiver  fitting.Fitting
		allocator *signal.PoolAllocator
	}
)

func (l Line) route(bufferSize int) (*route, error) {
	source, err := l.Source.allocate(componentContext(l.Context), bufferSize)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	prevProps := source.SignalProperties

	processors := make([]*Processor, 0, len(l.Processors))
	for i := range l.Processors {
		processor, err := l.Processors[i].allocate(componentContext(l.Context), bufferSize, prevProps)
		if err != nil {
			return nil, fmt.Errorf("processor: %w", err)
		}
		prevProps = processor.SignalProperties
		processors = append(processors, &processor)
	}

	sink, err := l.Sink.allocate(componentContext(l.Context), bufferSize, prevProps)
	if err != nil {
		return nil, fmt.Errorf("sink: %w", err)
	}

	return &route{
		context:    l.Context,
		source:     &source,
		processors: processors,
		sink:       &sink,
	}, nil
}

func (p *Pipe) addExecutors(r *route, routeIdx int) {
	if r.context.IsMutable() {
		if e, ok := p.executors[r.context]; ok {
			mle := e.(*multiLineExecutor)
			mle.Lines = append(mle.Lines, r.executor(mle.Destination, routeIdx))
			return
		}
		d := mutable.NewDestination()
		p.pusher.AddDestination(r.context, d)
		p.executors[r.context] = &multiLineExecutor{
			Context:     r.context,
			Destination: d,
			Lines:       []*lineExecutor{r.executor(d, routeIdx)},
		}
		return
	}

	d := mutable.NewDestination()
	r.source.dest = d
	p.pusher.AddDestination(r.source.Context, d)
	p.executors[r.source.Context] = r.source
	for i := range r.processors {
		p.pusher.AddDestination(r.processors[i].Context, d)
		p.executors[r.processors[i].Context] = r.processors[i]
	}
	p.pusher.AddDestination(r.sink.Context, d)
	p.executors[r.sink.Context] = r.sink
}

func (r *route) connect(bufferSize int) {
	fn := fitting.Async
	if r.context.IsMutable() {
		fn = fitting.Sync
	}
	r.source.connect(bufferSize, fn)
	prevOut := r.source.out
	for _, p := range r.processors {
		p.connect(bufferSize, fn, prevOut)
		prevOut = p.out
	}
	r.sink.connect(bufferSize, prevOut)
}

func (r *route) executor(d mutable.Destination, idx int) *lineExecutor {
	executors := make([]executor, 0, 2+len(r.processors))
	r.source.dest = d
	executors = append(executors, r.source)
	for i := range r.processors {
		executors = append(executors, r.processors[i])
	}
	executors = append(executors, r.sink)
	return &lineExecutor{
		route:     idx,
		executors: executors,
	}
}

func (r *route) prev(pos int) (SignalProperties, out) {
	if pos == 0 {
		return r.source.SignalProperties, r.source.out
	}
	p := r.processors[pos-1]
	return p.SignalProperties, p.out
}

func (fn SourceAllocatorFunc) allocate(ctx mutable.Context, bufferSize int) (Source, error) {
	c, err := fn(ctx, bufferSize)
	if err != nil {
		return Source{}, err
	}
	c.Context = ctx
	return c, nil
}

func (fn ProcessorAllocatorFunc) allocate(ctx mutable.Context, bufferSize int, prevProps SignalProperties) (Processor, error) {
	c, err := fn(ctx, bufferSize, prevProps)
	if err != nil {
		return Processor{}, err
	}
	c.Context = ctx
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

func (in *in) insert(out out) {
	in.receiver = out.sender
	in.allocator = out.allocator
}

func componentContext(lineCtx mutable.Context) mutable.Context {
	if lineCtx.IsMutable() {
		return lineCtx
	}
	return mutable.Mutable()
}
