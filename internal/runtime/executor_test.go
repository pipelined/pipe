package runtime_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"pipelined.dev/pipe/internal/runtime"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

var mockError = errors.New("mock error")

func TestHooks(t *testing.T) {
	testOk := func(t *testing.T) {
		ctx := context.Background()
		hookOk := func(context.Context) error {
			return nil
		}
		err := runtime.StartFunc(hookOk).Start(ctx)
		assertEqual(t, "start error", err, nil)
		err = runtime.FlushFunc(hookOk).Flush(ctx)
		assertEqual(t, "flush error", err, nil)
	}

	testNil := func(t *testing.T) {
		ctx := context.Background()
		var start runtime.StartFunc
		err := start.Start(ctx)
		assertEqual(t, "start error", err, nil)
		var flush runtime.FlushFunc
		err = flush.Flush(ctx)
		assertEqual(t, "flush error", err, nil)
	}

	t.Run("hooks ok", testOk)
	t.Run("hooks nil", testNil)
}

func TestSource(t *testing.T) {
	testOk := func(t *testing.T) {
		ctx := context.Background()
		link := runtime.SyncLink()
		e := runtime.Source{
			OutputPool: signal.GetPoolAllocator(2, 4, 4),
			Sender:     link,
			SourceFn: func(out signal.Floating) (int, error) {
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
	testExecutionError := func(t *testing.T) {
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
	testContextDone := func(t *testing.T) {
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
		assertEqual(t, "execute error", err, io.EOF)
	}
	testContextDoneOnSend := func(t *testing.T) {
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
		assertEqual(t, "execute error", err, io.EOF)
	}
	t.Run("source ok", testOk)
	t.Run("source mutations ok", testMutations)
	t.Run("source execution error", testExecutionError)
	t.Run("source context done", testContextDone)
	t.Run("source context done on send", testContextDoneOnSend)
}

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
			ProcessFn: func(in, out signal.Floating) error {
				signal.FloatingAsFloating(in, out)
				return nil
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
			ProcessFn: func(io, out signal.Floating) error {
				return mockError
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
			ProcessFn: func(io, out signal.Floating) error {
				return mockError
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
			ProcessFn: func(io, out signal.Floating) error {
				return nil
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

func TestSink(t *testing.T) {
	testOk := func(t *testing.T) {
		ctx := context.Background()
		link := runtime.SyncLink()
		e := runtime.Sink{
			Context:   mutable.Mutable(),
			InputPool: signal.GetPoolAllocator(2, 4, 4),
			Receiver:  link,
			SinkFn: func(in signal.Floating) error {
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

func TestLines(t *testing.T) {
	// ls := runtime.Lines{
	// 	Mutations: make(chan mutable.Mutations, 1),
	// 	Lines:     []runtime.Line{},
	// }
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
