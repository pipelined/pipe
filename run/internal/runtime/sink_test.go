package runtime_test

import (
	"context"
	"io"
	"testing"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/run/internal/runtime"
	"pipelined.dev/signal"
)

func TestSink(t *testing.T) {
	testOk := func(t *testing.T) {
		ctx := context.Background()
		link := runtime.SyncLink()
		e := runtime.Sink{
			Context:   mutable.Mutable(),
			InputPool: signal.GetPoolAllocator(2, 4, 4),
			Receiver:  link,
			SinkFunc: func(in signal.Floating) error {
				return nil
			},
		}
		var called bool
		var ms mutable.Mutations
		ms = ms.Put(e.Mutate(func() {
			called = true
		}))
		link.Send(ctx, runtime.Message{
			Signal:    e.InputPool.GetFloat64(),
			Mutations: ms,
		})
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, nil)
		result, ok := link.Receive(ctx)
		assertEqual(t, "result ok", ok, true)
		assertEqual(t, "result length", result.Signal.Len(), 8)
		assertEqual(t, "mutator executed", called, true)
	}
	testContextDone := func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		e := runtime.Sink{
			Receiver: runtime.AsyncLink(),
		}
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, io.EOF)
	}

	t.Run("sink ok", testOk)
	t.Run("sink context done", testContextDone)
}
