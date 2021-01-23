package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type (
	// Source is the executor for source component.
	Source struct {
		Mutations chan mutable.Mutations
		mutable.Context
		OutputPool *signal.PoolAllocator
		SourceFn   SourceFunc
		StartFunc
		FlushFunc
		Sender
	}
	// SourceFunc is a wrapper type of source closure.
	SourceFunc func(out signal.Floating) (int, error)

	// Processor is the executor for processor component.
	Processor struct {
		mutable.Context
		InputPool  *signal.PoolAllocator
		OutputPool *signal.PoolAllocator
		ProcessFn  ProcessFunc
		StartFunc
		FlushFunc
		Receiver
		Sender
	}
	// ProcessFunc is a wrapper type of processor closure.
	ProcessFunc func(in, out signal.Floating) error

	// Sink is the executor for processor component.
	Sink struct {
		mutable.Context
		InputPool *signal.PoolAllocator
		SinkFn    SinkFunc
		StartFunc
		FlushFunc
		Receiver
	}
	// SinkFunc is a wrapper type of sink closure.
	SinkFunc func(in signal.Floating) error
)

type (
	// Lines executes multiple lines in the same goroutine.
	Lines struct {
		Mutations chan mutable.Mutations
		Lines     []Line
	}

	// Line represents a sequence of components executors.
	Line struct {
		started   int
		Executors []Executor
	}
)

type (
	// StartFunc is a closure that triggers pipe component start hook.
	StartFunc func(ctx context.Context) error
	// FlushFunc is a closure that triggers pipe component start hook.
	FlushFunc func(ctx context.Context) error
)

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

// Run starts executor for source component.
func (e Source) Run(ctx context.Context) <-chan error {
	return start(ctx, e)
}

// Execute does a single iteration of source component. io.EOF is returned
// if context is done.
func (e Source) Execute(ctx context.Context) error {
	var ms mutable.Mutations
	select {
	case ms = <-e.Mutations:
		ms.ApplyTo(e.Context)
	case <-ctx.Done():
		e.Sender.Close()
		return io.EOF
	default:
	}

	out := e.OutputPool.GetFloat64()
	var (
		read int
		err  error
	)
	if read, err = e.SourceFn(out); err != nil {
		e.Sender.Close()
		out.Free(e.OutputPool)
		return err
	}
	if read != out.Length() {
		out = out.Slice(0, read)
	}

	if !e.Sender.Send(ctx, Message{Signal: out, Mutations: ms}) {
		e.Sender.Close()
		return io.EOF
	}
	return nil
}

// Run starts executor for processor component.
func (e Processor) Run(ctx context.Context) <-chan error {
	return start(ctx, e)
}

// Execute does a single iteration of processor component. io.EOF is
// returned if context is done.
func (e Processor) Execute(ctx context.Context) error {
	m, ok := e.Receiver.Receive(ctx)
	if !ok {
		e.Sender.Close()
		return io.EOF
	}
	m.Mutations.ApplyTo(e.Context)

	out := e.OutputPool.GetFloat64()
	if err := e.ProcessFn(m.Signal, out); err != nil {
		e.Sender.Close()
		return err
	}

	if !e.Sender.Send(ctx, Message{Signal: out, Mutations: m.Mutations}) {
		e.Sender.Close()
		out.Free(e.OutputPool)
		return io.EOF
	}
	m.Signal.Free(e.InputPool)
	return nil
}

// Run starts executor for sink component.
func (e Sink) Run(ctx context.Context) <-chan error {
	return start(ctx, e)
}

// Execute does a single iteration of sink component. io.EOF is returned if
// context is done.
func (e Sink) Execute(ctx context.Context) error {
	m, ok := e.Receiver.Receive(ctx)
	if !ok {
		return io.EOF
	}
	m.Mutations.ApplyTo(e.Context)

	err := e.SinkFn(m.Signal)
	m.Signal.Free(e.InputPool)
	return err
}

// Run starts executor for lines component.
func (e *Lines) Run(ctx context.Context) <-chan error {
	return start(ctx, e)
}

// Start calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (e *Lines) Start(ctx context.Context) error {
	var startErr lineErrors
	for i := range e.Lines {
		if err := e.Lines[i].start(ctx); err != nil {
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
		err = fmt.Errorf("error flushing lines during start error: %w", flushErr)
	}
	return err
}

// Flush flushes all lines.
func (e *Lines) Flush(ctx context.Context) error {
	var flushErr lineErrors
	for i := range e.Lines {
		if err := e.Lines[i].flush(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (e *Lines) Execute(ctx context.Context) error {
	for i := range e.Lines {
		if err := e.Lines[i].execute(ctx); err != nil {
			// TODO: handle EOF
			return err
		}
	}
	return nil
}

func (l *Line) execute(ctx context.Context) error {
	for _, e := range l.Executors {
		if err := e.Execute(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (l *Line) flush(ctx context.Context) error {
	var errs lineErrors
	for i := 0; i < l.started; i++ {
		if err := l.Executors[i].Flush(ctx); err != nil {
			errs = append(errs, err)
			break
		}
	}
	return errs.ret()
}

func (l *Line) start(ctx context.Context) error {
	var errs lineErrors
	for _, e := range l.Executors {
		if err := e.Start(ctx); err != nil {
			errs = append(errs, err)
			break
		}
		l.started++
	}
	return errs.ret()
}

type lineErrors []error

func (e lineErrors) Error() string {
	s := []string{}
	for _, se := range e {
		s = append(s, se.Error())
	}
	return strings.Join(s, ",")
}

// ret returns untyped nil if error is list is empty.
func (e lineErrors) ret() error {
	if len(e) > 0 {
		return e
	}
	return nil
}
