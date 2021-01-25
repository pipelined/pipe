package runtime

import (
	"context"
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

// Processor is the executor for processor component.
type Processor struct {
	mutable.Context
	InputPool  *signal.PoolAllocator
	OutputPool *signal.PoolAllocator
	pipe.ProcessFunc
	StartFunc
	FlushFunc
	Receiver
	Sender
}

// ProcessExecutor returns executor for processor component.
func ProcessExecutor(p pipe.Processor, input, output *signal.PoolAllocator, receiver, sender Link) Processor {
	return Processor{
		Context:     p.Context,
		InputPool:   input,
		OutputPool:  output,
		ProcessFunc: p.ProcessFunc,
		StartFunc:   StartFunc(p.StartFunc),
		FlushFunc:   FlushFunc(p.FlushFunc),
		Receiver:    receiver,
		Sender:      sender,
	}
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
	if processed, err := e.ProcessFunc(m.Signal, out); err != nil {
		e.Sender.Close()
		return err
	} else if processed != e.OutputPool.Length {
		out = out.Slice(0, processed)
	}

	if !e.Sender.Send(ctx, Message{Signal: out, Mutations: m.Mutations}) {
		e.Sender.Close()
		out.Free(e.OutputPool)
		return io.EOF
	}
	m.Signal.Free(e.InputPool)
	return nil
}
