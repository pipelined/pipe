package runtime

import (
	"context"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type Source struct {
	Mutations chan mutable.Mutations
	mutable.Context
	OutputPool *signal.PoolAllocator
	SourceFn   SourceFunc
	StartFunc
	FlushFunc
	Sender
}

func (e Source) Execute(ctx context.Context) error {
	var ms mutable.Mutations
	select {
	case ms = <-e.Mutations:
		ms.ApplyTo(e.Context)
	case <-ctx.Done():
		e.Sender.Close()
		return ErrContextDone
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
		return ErrContextDone
	}
	return nil
}
