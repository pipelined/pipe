package pipe

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/fitting"
	"pipelined.dev/pipe/mutable"
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
)

type (
	// Pipe is a graph formed with multiple lines of bound DSP components.
	Pipe struct {
		bufferSize int
		Lines      []*Line
	}

	// Line bounds the routing to the context and buffer size.
	Line struct {
		bufferSize int
		mutable.Context
		Source
		Processors []Processor
		Sink
	}

	// Source is a source of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Source struct {
		mutations chan mutable.Mutations
		mutable.Context
		Output SignalProperties
		SourceFunc
		StartFunc
		FlushFunc
		out output
	}

	// SourceFunc takes the output buffer and fills it with a signal data.
	// If no data is available, io.EOF should be returned.
	SourceFunc func(out signal.Floating) (int, error)

	// Processor is a mutator of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Processor struct {
		mutable.Context
		Output SignalProperties
		ProcessFunc
		StartFunc
		FlushFunc
		in  input
		out output
	}

	// ProcessFunc takes the input buffer, applies processing logic and
	// writes the result into output buffer.
	ProcessFunc func(in, out signal.Floating) (int, error)

	// Sink is a destination of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Sink struct {
		mutable.Context
		Output SignalProperties
		SinkFunc
		StartFunc
		FlushFunc
		in input
	}

	// SinkFunc takes the input buffer and writes that to the underlying
	// destination.
	SinkFunc func(in signal.Floating) error

	// StartFunc provides a hook to flush all buffers for the component.
	StartFunc func(ctx context.Context) error
	// FlushFunc provides a hook to flush all buffers for the component or
	// execute any other form of finalization logic.
	FlushFunc func(ctx context.Context) error
)

type (
	output struct {
		fitting.Sender
		*signal.PoolAllocator
	}

	input struct {
		fitting.Receiver
		*signal.PoolAllocator
	}
)

// New returns a new Pipe that binds multiple lines using the provided
// buffer size.
func New(bufferSize int, routes ...Routing) (*Pipe, error) {
	if len(routes) == 0 {
		panic("pipe without lines")
	}
	// context for pipe binding.
	// will be cancelled if any binding failes.
	lines := make([]*Line, 0, len(routes))
	for i := range routes {
		l, err := routes[i].line(bufferSize)
		if err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}

	return &Pipe{
		bufferSize: bufferSize,
		Lines:      lines,
	}, nil
}

// AddLine creates the line for provied route and adds it to the pipe.
func (p *Pipe) AddLine(r Routing) (*Line, error) {
	l, err := r.line(p.bufferSize)
	if err != nil {
		return nil, err
	}
	p.Lines = append(p.Lines, l)
	return l, nil
}

func (l *Line) prev(pos int) mutable.Context {
	if pos == 0 {
		return l.Source.Context
	}
	return l.Processors[pos-1].Context
}

func (l *Line) next(pos int) mutable.Context {
	if pos+1 == len(l.Processors) {
		return l.Sink.Context
	}
	return l.Processors[pos+1].Context
}

func (fn SourceAllocatorFunc) allocate(mctx mutable.Context, bufferSize int) (Source, error) {
	c, err := fn(mctx, bufferSize)
	if err == nil {
		c.Context = mctx
	}
	return c, err
}

func (fn ProcessorAllocatorFunc) allocate(mctx mutable.Context, bufferSize int, input SignalProperties) (Processor, error) {
	c, err := fn(mctx, bufferSize, input)
	if err == nil {
		c.Context = mctx
	}
	return c, err
}

func (fn SinkAllocatorFunc) allocate(mctx mutable.Context, bufferSize int, input SignalProperties) (Sink, error) {
	c, err := fn(mctx, bufferSize, input)
	if err == nil {
		c.Context = mctx
	}
	return c, err
}

func (r Routing) line(bufferSize int) (*Line, error) {
	source, err := r.Source.allocate(componentContext(r.Context), bufferSize)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	input := source.Output
	processors := make([]Processor, 0, len(r.Processors))
	for i := range r.Processors {
		processor, err := r.Processors[i].allocate(componentContext(r.Context), bufferSize, input)
		if err != nil {
			return nil, fmt.Errorf("processor: %w", err)
		}
		processors = append(processors, processor)
		input = processor.Output
	}

	sink, err := r.Sink.allocate(componentContext(r.Context), bufferSize, input)
	if err != nil {
		return nil, fmt.Errorf("sink: %w", err)
	}

	return &Line{
		Context:    r.Context,
		bufferSize: bufferSize,
		Source:     source,
		Processors: processors,
		Sink:       sink,
	}, nil
}

func componentContext(routeContext mutable.Context) mutable.Context {
	if routeContext.IsMutable() {
		return routeContext
	}
	return mutable.Mutable()
}

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

func (l *Line) SourceOutputPool() *signal.PoolAllocator {
	return l.Source.Output.PoolAllocator(l.bufferSize)
}

func (l *Line) SinkOutputPool() *signal.PoolAllocator {
	return l.Sink.Output.PoolAllocator(l.bufferSize)
}

func (l *Line) ProcessorOutputPool(i int) *signal.PoolAllocator {
	return l.Processors[i].Output.PoolAllocator(l.bufferSize)
}

func (sp SignalProperties) PoolAllocator(bufferSize int) *signal.PoolAllocator {
	return signal.GetPoolAllocator(sp.Channels, bufferSize, bufferSize)
}
