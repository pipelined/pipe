package runtime

import (
	"context"
	"io"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type Sink struct {
	mutable.Context
	InputPool *signal.PoolAllocator
	SinkFn    SinkFunc
	StartFunc
	FlushFunc
	Receiver
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
