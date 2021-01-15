package runtime_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pipelined.dev/pipe/internal/runtime"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

var mockError = errors.New("mock error")

func TestSource(t *testing.T) {
	var (
		errHook = errors.New("hook failed")
		hookOk  = func(context.Context) error {
			return nil
		}
		_ = func(context.Context) error {
			return errHook
		}
	)

	testOk := func() func(*testing.T) {
		return func(t *testing.T) {
			ctx := context.Background()
			link := runtime.SyncLink()
			e := runtime.Source{
				OutputPool: signal.GetPoolAllocator(2, 4, 4),
				SourceFn: func(out signal.Floating) (int, error) {
					return signal.WriteStripedFloat64(
						[][]float64{
							{1, 2, 3},
							{11, 12, 13},
						},
						out,
					), nil
				},
				StartFunc: hookOk,
				FlushFunc: hookOk,
				Sender:    link,
			}
			var err error
			err = e.Start(ctx)
			assertEqual(t, "start error", err, nil)
			err = e.Execute(ctx)
			assertEqual(t, "execute error", err, nil)
			err = e.Flush(ctx)
			assertEqual(t, "flush error", err, nil)
			result, ok := link.Receive(ctx)
			assertEqual(t, "result ok", ok, true)
			assertEqual(t, "result length", result.Signal.Len(), 6)
		}
	}
	testNilHooks := func() func(*testing.T) {
		return func(t *testing.T) {
			ctx := context.Background()
			e := runtime.Source{}
			err := e.Start(ctx)
			assertEqual(t, "start error", err, nil)
			err = e.Flush(ctx)
			assertEqual(t, "flush error", err, nil)
		}
	}
	testMutations := func() func(*testing.T) {
		return func(t *testing.T) {
			ctx := context.Background()
			link := runtime.SyncLink()
			e := runtime.Source{
				Context:    mutable.Mutable(),
				Mutations:  make(chan mutable.Mutations, 1),
				OutputPool: signal.GetPoolAllocator(2, 4, 4),
				SourceFn: func(out signal.Floating) (int, error) {
					return 0, nil
				},
				Sender: link,
			}
			var called bool
			var ms mutable.Mutations
			e.Mutations <- ms.Put(e.Mutate(func() {
				called = true
			}))
			err := e.Execute(ctx)
			assertEqual(t, "execute error", err, nil)
			result, ok := link.Receive(ctx)
			assertEqual(t, "result ok", ok, true)
			assertEqual(t, "mutator executed", called, true)
			assertEqual(t, "mutator removed", len(result.Mutations), 0)
		}
	}
	testExecutionError := func() func(*testing.T) {
		return func(t *testing.T) {
			ctx := context.Background()
			e := runtime.Source{
				OutputPool: signal.GetPoolAllocator(2, 4, 4),
				SourceFn: func(out signal.Floating) (int, error) {
					return 0, mockError
				},
				Sender: runtime.SyncLink(),
			}
			err := e.Execute(ctx)
			assertEqual(t, "execute error", err, mockError)
		}
	}
	testContextDone := func() func(*testing.T) {
		return func(t *testing.T) {
			ctx, cancelFn := context.WithCancel(context.Background())
			cancelFn()
			e := runtime.Source{
				OutputPool: signal.GetPoolAllocator(2, 4, 4),
				SourceFn: func(out signal.Floating) (int, error) {
					return 0, mockError
				},
				Sender: runtime.AsyncLink(),
			}
			err := e.Execute(ctx)
			assertEqual(t, "execute error", err, runtime.ErrContextDone)
		}
	}
	testContextDoneOnSend := func() func(*testing.T) {
		return func(t *testing.T) {
			ctx, cancelFn := context.WithCancel(context.Background())
			link := runtime.AsyncLink()
			e := runtime.Source{
				Context:    mutable.Mutable(),
				Mutations:  make(chan mutable.Mutations, 1),
				OutputPool: signal.GetPoolAllocator(2, 4, 4),
				SourceFn: func(out signal.Floating) (int, error) {
					return 0, nil
				},
				Sender: link,
			}
			// to fill the channel buffer
			link.Send(ctx, runtime.Message{})
			var ms mutable.Mutations
			e.Mutations <- ms.Put(e.Mutate(func() {
				cancelFn()
			}))
			err := e.Execute(ctx)
			assertEqual(t, "execute error", err, runtime.ErrContextDone)
		}
	}
	t.Run("source ok", testOk())
	t.Run("source nil hooks", testNilHooks())
	t.Run("source mutations ok", testMutations())
	t.Run("source execution error", testExecutionError())
	t.Run("source context done", testContextDone())
	t.Run("source context done on send", testContextDoneOnSend())
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
