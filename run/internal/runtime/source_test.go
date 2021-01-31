package runtime_test

import (
	"context"
	"io"
	"testing"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/run/internal/runtime"
	"pipelined.dev/signal"
)

func TestSource(t *testing.T) {
	testOk := func(t *testing.T) {
		ctx := context.Background()
		link := runtime.SyncLink()
		e := runtime.Source{
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			Sender:     link,
			SourceFunc: func(out signal.Floating) (int, error) {
				return signal.WriteStripedFloat64(
					[][]float64{
						{1, 2, 3},
						{11, 12, 13},
					},
					out,
				), nil
			},
		}
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, nil)
		result, ok := link.Receive(ctx)
		assertEqual(t, "result ok", ok, true)
		assertEqual(t, "result length", result.Signal.Len(), 6)
	}
	testMutations := func(t *testing.T) {
		ctx := context.Background()
		link := runtime.SyncLink()
		e := runtime.Source{
			Context:    mutable.Mutable(),
			Mutations:  make(chan mutable.Mutations, 1),
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			SourceFunc: func(out signal.Floating) (int, error) {
				return 0, nil
			},
			Sender: link,
		}
		var called bool
		var ms mutable.Mutations
		e.Mutations <- ms.Put(e.Mutate(func() error {
			called = true
			return nil
		}))
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, nil)
		result, ok := link.Receive(ctx)
		assertEqual(t, "result ok", ok, true)
		assertEqual(t, "mutator executed", called, true)
		assertEqual(t, "mutator removed", len(result.Mutations), 0)
	}
	testExecutionError := func(t *testing.T) {
		ctx := context.Background()
		e := runtime.Source{
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			SourceFunc: func(out signal.Floating) (int, error) {
				return 0, mockError
			},
			Sender: runtime.SyncLink(),
		}
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, mockError)
	}
	testContextDone := func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		e := runtime.Source{
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			SourceFunc: func(out signal.Floating) (int, error) {
				return 0, mockError
			},
			Sender: runtime.AsyncLink(),
		}
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, io.EOF)
	}
	testContextDoneOnSend := func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(context.Background())
		link := runtime.AsyncLink()
		e := runtime.Source{
			Context:    mutable.Mutable(),
			Mutations:  make(chan mutable.Mutations, 1),
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			SourceFunc: func(out signal.Floating) (int, error) {
				return 0, nil
			},
			Sender: link,
		}
		// to fill the channel buffer
		link.Send(ctx, runtime.Message{})
		var ms mutable.Mutations
		e.Mutations <- ms.Put(e.Mutate(func() error {
			cancelFn()
			return nil
		}))
		err := e.Execute(ctx)
		assertEqual(t, "execute error", err, io.EOF)
	}
	t.Run("source ok", testOk)
	t.Run("source mutations ok", testMutations)
	t.Run("source execution error", testExecutionError)
	t.Run("source context done", testContextDone)
	t.Run("source context done on send", testContextDoneOnSend)
}
