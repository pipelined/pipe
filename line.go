package pipe

import (
	"context"
	"fmt"
	"io"
	"strings"

	"pipelined.dev/pipe/internal/fitting"
	"pipelined.dev/pipe/internal/run"
	"pipelined.dev/pipe/mutable"
)

type (
	// Routing defines sequence of DSP components allocators. It has a
	// single source, zero or many processors and single sink.
	Routing struct {
		mutable.Context
		Source     SourceAllocatorFunc
		Processors []ProcessorAllocatorFunc
		Sink       SinkAllocatorFunc
	}

	// Executor bounds multiple executors to the same mutations context.
	executor struct {
		Executors []run.Executor
	}

	// execErrors wraps errors that might occure when multiple executors
	// are failing.
	execErrors []error
)

// executor binds routing components together. Line is a set of components
// ready for execution. If executor is async then source context is returned.
func (r Routing) executor(mutations chan mutable.Mutations, bufferSize int, fitFn fitting.New) (executor, mutable.Context, error) {
	executors := make([]run.Executor, 0, 2+len(r.Processors))
	source, err := r.Source.allocate(componentContext(r.Context), bufferSize, fitFn)
	if err != nil {
		return executor{}, mutable.Immutable(), fmt.Errorf("source: %w", err)
	}
	source.mutations = mutations
	executors = append(executors, source)

	var (
		input      = source.out
		inputProps = source.SignalProperties
	)
	for i := range r.Processors {
		processor, err := r.Processors[i].allocate(componentContext(r.Context), bufferSize, inputProps, fitFn)
		if err != nil {
			return executor{}, mutable.Immutable(), fmt.Errorf("processor: %w", err)
		}
		processor.in, input = input, processor.out
		inputProps = processor.SignalProperties
		executors = append(executors, processor)
	}

	sink, err := r.Sink.allocate(componentContext(r.Context), bufferSize, inputProps)
	if err != nil {
		return executor{}, mutable.Immutable(), fmt.Errorf("sink: %w", err)
	}
	sink.in = input
	executors = append(executors, sink)

	return executor{Executors: executors}, source.Context, nil
}

func (fn SourceAllocatorFunc) allocate(ctx mutable.Context, bufferSize int, fitFn fitting.New) (Source, error) {
	c, err := fn(ctx, bufferSize)
	if err != nil {
		return Source{}, err
	}
	c.Context = ctx
	c.out = connector{
		Fitting:       fitFn(),
		PoolAllocator: c.SignalProperties.poolAllocator(bufferSize),
	}
	return c, nil
}

func (fn ProcessorAllocatorFunc) allocate(ctx mutable.Context, bufferSize int, input SignalProperties, fitFn fitting.New) (Processor, error) {
	c, err := fn(ctx, bufferSize, input)
	if err != nil {
		return Processor{}, err
	}
	c.Context = ctx
	c.out = connector{
		Fitting:       fitFn(),
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

// Start calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (e executor) Start(ctx context.Context) error {
	var startErr execErrors
	for _, ex := range e.Executors {
		if err := ex.Start(ctx); err != nil {
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
	flushErr := e.Flush(ctx)
	if flushErr != nil {
		err = fmt.Errorf("error flushing lines: %w during start error: %v", flushErr, err)
	}
	return err
}

// Flush flushes all lines.
func (e executor) Flush(ctx context.Context) error {
	var flushErr execErrors
	for _, ex := range e.Executors {
		if err := ex.Flush(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (e executor) Execute(ctx context.Context) error {
	var err error
	for i := 0; i < len(e.Executors); {
		if err = e.Executors[i].Execute(ctx); err == nil {
			i++
			continue
		}
		if err == io.EOF {
			if flushErr := e.Executors[i].Flush(ctx); flushErr != nil {
				return flushErr
			}
			e.Executors = append(e.Executors[:i], e.Executors[i+1:]...)
			if len(e.Executors) > 0 {
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
