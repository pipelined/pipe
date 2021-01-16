package runtime

import (
	"context"
	"io"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type (
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
