package runtime

import (
	"context"
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

// Source is the executor for source component.
type Source struct {
	Mutations chan mutable.Mutations
	mutable.Context
	OutputPool *signal.PoolAllocator
	pipe.SourceFunc
	StartFunc
	FlushFunc
	Sender
}

// SourceExecutor returns executor for source component.
func SourceExecutor(s pipe.Source, mc chan mutable.Mutations, output *signal.PoolAllocator, sender Link) Source {
	return Source{
		Mutations:  mc,
		Context:    s.Context,
		OutputPool: output,
		SourceFunc: s.SourceFunc,
		StartFunc:  StartFunc(s.StartFunc),
		FlushFunc:  FlushFunc(s.FlushFunc),
		Sender:     sender,
	}
}

// Run starts executor for source component.
// func (e Source) Run(ctx context.Context) <-chan error {
// 	return Run(ctx, e)
// }

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
	if read, err = e.SourceFunc(out); err != nil {
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
