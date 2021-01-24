package runtime

import (
	"context"
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

// Sink is the executor for processor component.
type Sink struct {
	mutable.Context
	InputPool *signal.PoolAllocator
	pipe.SinkFunc
	StartFunc
	FlushFunc
	Receiver
}

// SinkExecutor returns executor for sink component.
func SinkExecutor(s pipe.Sink, input *signal.PoolAllocator, receiver Link) Sink {
	return Sink{
		Context:   s.Context,
		InputPool: input,
		SinkFunc:  s.SinkFunc,
		StartFunc: StartFunc(s.StartFunc),
		FlushFunc: FlushFunc(s.FlushFunc),
		Receiver:  receiver,
	}
}

// Run starts executor for sink component.
func (e Sink) Run(ctx context.Context) <-chan error {
	return Start(ctx, e)
}

// Execute does a single iteration of sink component. io.EOF is returned if
// context is done.
func (e Sink) Execute(ctx context.Context) error {
	m, ok := e.Receiver.Receive(ctx)
	if !ok {
		return io.EOF
	}
	m.Mutations.ApplyTo(e.Context)

	err := e.SinkFunc(m.Signal)
	m.Signal.Free(e.InputPool)
	return err
}
