package runtime_test

import (
	"context"
	"io"
	"testing"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/run/internal/runtime"
	"pipelined.dev/signal"
)

func TestProcessor(t *testing.T) {
	testOk := func(t *testing.T) {
		ctx := context.Background()
		receiver, sender := runtime.SyncLink(), runtime.SyncLink()
		pool := signal.GetPoolAllocator(2, 4, 4)
		e := runtime.Processor{
			Context:    mutable.Mutable(),
			InputPool:  pool,
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			Receiver:   receiver,
			Sender:     sender,
			ProcessFunc: func(in, out signal.Floating) (int, error) {
				return signal.FloatingAsFloating(in, out), nil
			},
		}
		var called bool
		var ms mutable.Mutations
		ms = ms.Put(e.Mutate(func() {
			called = true
		}))
		receiver.Send(ctx,
			runtime.Message{
				Signal:    pool.GetFloat64(),
				Mutations: ms,
			},
		)
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, nil)
		result, ok := sender.Receive(ctx)
		assertEqual(t, "result ok", ok, true)
		assertEqual(t, "result length", result.Signal.Len(), 8)
		assertEqual(t, "mutator executed", called, true)
		assertEqual(t, "mutator removed", len(result.Mutations), 0)
	}
	testExecutionError := func(t *testing.T) {
		ctx := context.Background()
		receiver := runtime.AsyncLink()
		e := runtime.Processor{
			InputPool:  signal.GetPoolAllocator(2, 4, 4),
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			ProcessFunc: func(in, out signal.Floating) (int, error) {
				return 0, mockError
			},
			Receiver: receiver,
			Sender:   runtime.SyncLink(),
		}
		receiver.Send(ctx, runtime.Message{})
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, mockError)
	}
	testContextDone := func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		e := runtime.Processor{
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			InputPool:  signal.GetPoolAllocator(2, 4, 4),
			ProcessFunc: func(in, out signal.Floating) (int, error) {
				return 0, mockError
			},
			Receiver: runtime.AsyncLink(),
			Sender:   runtime.AsyncLink(),
		}
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, io.EOF)
	}

	testContextDoneOnSend := func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(context.Background())
		receiver, sender := runtime.AsyncLink(), runtime.AsyncLink()
		e := runtime.Processor{
			Context:    mutable.Mutable(),
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			InputPool:  signal.GetPoolAllocator(2, 4, 4),
			ProcessFunc: func(in, out signal.Floating) (int, error) {
				return 0, nil
			},
			Receiver: receiver,
			Sender:   sender,
		}
		var ms mutable.Mutations
		ms = ms.Put(e.Mutate(func() {
			cancelFn()
		}))
		receiver.Send(ctx, runtime.Message{
			Signal:    e.InputPool.GetFloat64(),
			Mutations: ms,
		})
		// to fill the buffer and cause the context done
		sender.Send(ctx, runtime.Message{})
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, io.EOF)
	}

	t.Run("processor ok", testOk)
	t.Run("processor error", testExecutionError)
	t.Run("processor context done", testContextDone)
	t.Run("processor context done on send", testContextDoneOnSend)
}
