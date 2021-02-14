package pipe

import (
	"context"
	"fmt"
	"io"
	"strings"

	"pipelined.dev/pipe/internal/fitting"
	"pipelined.dev/pipe/internal/run"
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

	LineRunner struct {
		mutable.Context
		started   int
		executors []run.Executor
	}

	MultilineRunner struct {
		mutable.Context
		Lines []*LineRunner
	}

	// execErrors wraps errors that might occure when multiple executors
	// are failing.
	execErrors []error
)

func (r *LineRunner) run(ctx context.Context, merger *errorMerger) {
	r.Bind()
	for _, e := range r.executors {
		merger.add(run.Run(ctx, e))
	}
}

func (r *MultilineRunner) run(ctx context.Context, merger *errorMerger) {
	r.Bind()
	merger.add(run.Run(ctx, r))
}

func (r *LineRunner) Bind() {
	e := r.executors[0].(Source)
	e.out.Sender.Bind()

	for i := 1; i < len(r.executors)-1; i++ {
		e := r.executors[i].(Processor)
		e.out.Sender.Bind()
	}
}

func (r *MultilineRunner) Bind() {
	for i := range r.Lines {
		r.Lines[i].Bind()
	}
}

// Runner binds routing components together. Line is a set of components
// ready for execution. If Runner is async then source context is returned.
func (r Line) Runner(bufferSize int, mutations chan mutable.Mutations) (*LineRunner, error) {
	fitFn := fitting.Async
	if r.Context.IsMutable() {
		fitFn = fitting.Sync
	}
	executors := make([]run.Executor, 0, 2+len(r.Processors))
	source, err := r.Source.allocate(componentContext(r.Context), bufferSize)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	source.mutations = mutations

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
	for i := range r.Processors {
		processor, err := r.Processors[i].allocate(componentContext(r.Context), bufferSize, link.SignalProperties)
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

	sink, err := r.Sink.allocate(componentContext(r.Context), bufferSize, link.SignalProperties)
	if err != nil {
		return nil, fmt.Errorf("sink: %w", err)
	}
	sink.in.PoolAllocator = link.PoolAllocator
	sink.in.Receiver = link.Fitting
	executors = append(executors, sink)

	return &LineRunner{
		Context:   source.Context,
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

// Execute all components of the line one-by-one.
func (r *LineRunner) execute(ctx context.Context) error {
	var err error
	for _, e := range r.executors {
		if err = e.Execute(ctx); err == nil {
			continue
		}
		if err == io.EOF {
			// continue execution to propagate EOF
			continue
		}
		return err
	}
	// if no other errors were met, EOF will be returned
	return err
}

func (r *LineRunner) flush(ctx context.Context) error {
	var errs execErrors
	for i := 0; i < r.started; i++ {
		if err := r.executors[i].Flush(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errs.ret()
}

func (r *LineRunner) start(ctx context.Context) error {
	var errs execErrors
	for _, e := range r.executors {
		if err := e.Start(ctx); err != nil {
			errs = append(errs, err)
			break
		}
		r.started++
	}
	return errs.ret()
}

// Start calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (r *MultilineRunner) Start(ctx context.Context) error {
	var startErr execErrors
	for i := range r.Lines {
		if err := r.Lines[i].start(ctx); err != nil {
			startErr = append(startErr, err)
			break
		}
	}

	// all started smooth
	if len(startErr) == 0 {
		return nil
	}
	// wrap start error
	err := fmt.Errorf("error starting lines: %w", startErr.ret())
	// need to flush sucessfully started components
	flushErr := r.Flush(ctx)
	if flushErr != nil {
		err = fmt.Errorf("error flushing lines: %w during start error: %v", flushErr, err)
	}
	return err
}

// Flush flushes all lines.
func (r *MultilineRunner) Flush(ctx context.Context) error {
	var flushErr execErrors
	for _, l := range r.Lines {
		if err := l.flush(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (r *MultilineRunner) Execute(ctx context.Context) error {
	var err error
	for i := 0; i < len(r.Lines); {
		if err = r.Lines[i].execute(ctx); err == nil {
			i++
			continue
		}
		if err == io.EOF {
			if flushErr := r.Lines[i].flush(ctx); flushErr != nil {
				return flushErr
			}
			r.Lines = append(r.Lines[:i], r.Lines[i+1:]...)
			if len(r.Lines) > 0 {
				continue
			}
		}
		return err
	}
	return nil
}

func (e execErrors) Error() string {
	s := []string{}
	for _, se := range e {
		s = append(s, se.Error())
	}
	return strings.Join(s, ",")
}

// ret returns untyped nil if error is list is empty.
func (e execErrors) ret() error {
	if len(e) > 0 {
		return e
	}
	return nil
}
