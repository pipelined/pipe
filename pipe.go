package pipe

import (
	"context"
	"fmt"

	"pipelined.dev/signal"

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
		Source
		Processors []Processor
		Sink
	}

	// Source is a source of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Source struct {
		mctx   mutable.Context
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
		mctx   mutable.Context
		Output SignalProperties
		ProcessFunc
		StartFunc
		FlushFunc
	}

	// ProcessFunc takes the input buffer, applies processing logic and
	// writes the result into output buffer.
	ProcessFunc func(in, out signal.Floating) error

	// Sink is a destination of signal data. Optinaly, mutability can be
	// provided to handle mutations and flush hook to handle resource clean
	// up.
	Sink struct {
		mctx   mutable.Context
		Output SignalProperties
		SinkFunc
		StartFunc
		FlushFunc
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

// InsertProcessor inserts the processor to the line. Pos is the index
// where processor should be inserted relatively to other processors i.e:
// pos 0 means that new processor will be inserted right after the source.
func (l *Line) InsertProcessor(pos int, fn ProcessorAllocatorFunc) error {
	var inputProps SignalProperties
	if pos == 0 {
		inputProps = l.Source.Output
	} else {
		inputProps = l.Processors[pos-1].Output
	}

	// allocate new processor
	m := mutable.Mutable()
	proc, err := fn(m, l.bufferSize, inputProps)
	if err != nil {
		return err
	}
	proc.mctx = m
	// append processor
	l.Processors = append(l.Processors, Processor{})
	copy(l.Processors[pos+1:], l.Processors[pos:])
	l.Processors[pos] = proc
	return nil
}

func (l *Line) prev(pos int) mutable.Context {
	if pos == 0 {
		return l.Source.mctx
	}
	return l.Processors[pos-1].mctx
}

func (l *Line) next(pos int) mutable.Context {
	if pos+1 == len(l.Processors) {
		return l.Sink.mctx
	}
	return l.Processors[pos+1].mctx
}

func (r Routing) line(bufferSize int) (*Line, error) {
	m := mutable.Mutable()
	source, err := r.Source(m, bufferSize)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	source.mctx = m

	input := source.Output
	processors := make([]Processor, 0, len(r.Processors))
	for i := range r.Processors {
		m = mutable.Mutable()
		processor, err := r.Processors[i](m, bufferSize, input)
		if err != nil {
			return nil, fmt.Errorf("processor: %w", err)
		}
		processor.mctx = m
		processors = append(processors, processor)
		input = processor.Output
	}

	m = mutable.Mutable()
	sink, err := r.Sink(m, bufferSize, input)
	if err != nil {
		return nil, fmt.Errorf("sink: %w", err)
	}
	sink.mctx = m

	return &Line{
		bufferSize: bufferSize,
		Source:     source,
		Processors: processors,
		Sink:       sink,
	}, nil
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorAllocatorFunc) []ProcessorAllocatorFunc {
	return processors
}

func (sp SignalProperties) poolAllocator(bufferSize int) *signal.PoolAllocator {
	return signal.GetPoolAllocator(sp.Channels, bufferSize, bufferSize)
}
