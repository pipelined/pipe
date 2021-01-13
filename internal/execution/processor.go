package execution

import (
	"context"
	"io"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type Processor struct {
	mutable.Context
	InputPool  *signal.PoolAllocator
	OutputPool *signal.PoolAllocator
	ProcessFn  ProcessFunc
	StartFunc
	FlushFunc
	Receiver
	Sender
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
		return ErrContextDone
	}
	m.Signal.Free(e.InputPool)
	return nil
}
