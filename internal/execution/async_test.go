package execution_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/pipe/mutable"

	"pipelined.dev/signal"
)

var testError = errors.New("test runner error")

const (
	bufferSize = 1024
	channels   = 1
)

func TestSource(t *testing.T) {
	setupSource := func(sourceAllocator pipe.SourceAllocatorFunc, mutationsChan chan mutable.Mutations) execution.Starter {
		m := mutable.Mutable()
		source, _ := sourceAllocator(m, bufferSize)
		return execution.Starter(
			mutationsChan, m,
			signal.GetPoolAllocator(source.Output.Channels, bufferSize, bufferSize),
			execution.SourceFunc(source.SourceFunc),
			execution.HookFunc(source.StartFunc),
			execution.HookFunc(source.FlushFunc),
		)
	}
	assertSource := func(t *testing.T, mockSource *mock.Source, out <-chan execution.Message, errs <-chan error) {
		t.Helper()
		received := 0
		for buf := range out {
			received = received + buf.Signal.Length()
		}
		for err := range errs {
			assertEqual(t, "error", errors.Unwrap(err), testError)
		}
		assertEqual(t, "source samples", received, mockSource.Limit)
		assertEqual(t, "source flushed", mockSource.Flushed, true)
		return
	}
	testSource := func(ctx context.Context, mockSource mock.Source) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			r := setupSource(mockSource.Source(), nil)
			errs := r.Run(ctx)
			assertSource(t, &mockSource, r.Out(), errs)
		}
	}
	testContextDone := func(mockSource mock.Source) func(*testing.T) {
		t.Helper()
		cancelCtx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		return testSource(cancelCtx, mockSource)
	}

	testMutationError := func(ctx context.Context, mockSource mock.Source) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			mutationsChan := make(chan mutable.Mutations, 1)
			r := setupSource(mockSource.Source(), mutationsChan)
			mutationsChan <- mutable.Mutations{}.Put(mockSource.MockMutation())
			errs := r.Run(ctx)
			assertSource(t, &mockSource, r.Out(), errs)
		}
	}

	t.Run("ok", testSource(
		context.Background(),
		mock.Source{
			Channels: 1,
			Limit:    10*bufferSize + 1,
		},
	))
	t.Run("error", testSource(
		context.Background(),
		mock.Source{
			ErrorOnCall: testError,
			Channels:    1,
			Limit:       0,
		},
	))
	t.Run("flush error", testSource(
		context.Background(),
		mock.Source{
			Flusher: mock.Flusher{
				ErrorOnFlush: testError,
			},
			Channels: 1,
			Limit:    10 * bufferSize,
		},
	))

	t.Run("context done", testContextDone(
		mock.Source{
			Channels: 1,
			Limit:    0,
		},
	))
	t.Run("mutation error", testMutationError(
		context.Background(),
		mock.Source{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
			Channels: 1,
			Limit:    0,
		},
	))
}

func TestProcessor(t *testing.T) {
	setupRunner := func(processorAllocator pipe.ProcessorAllocatorFunc, alloc signal.Allocator) (execution.Starter, chan<- execution.Message) {
		m := mutable.Mutable()
		processor, _ := processorAllocator(m, bufferSize, pipe.SignalProperties{Channels: channels})
		in := make(chan execution.Message, 1)
		return execution.Processor(
			m,
			in,
			signal.GetPoolAllocator(processor.Output.Channels, bufferSize, bufferSize),
			signal.GetPoolAllocator(processor.Output.Channels, bufferSize, bufferSize),
			execution.ProcessFunc(processor.ProcessFunc),
			execution.HookFunc(processor.StartFunc),
			execution.HookFunc(processor.FlushFunc),
		), in
	}
	testProcessor := func(ctx context.Context, mockProcessor mock.Processor) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			alloc := signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}
			r, in := setupRunner(mockProcessor.Processor(), alloc)
			errc := r.Run(ctx)

			// test mutations only for mutable
			in <- execution.Message{
				Signal:    alloc.Float64(),
				Mutations: mutable.Mutations{}.Put(mockProcessor.MockMutation()),
			}
			close(in)
			for msg := range r.Out() {
				assertEqual(t, "samples", msg.Signal.Length(), alloc.Length)
			}
			for err := range errc {
				assertEqual(t, "error", errors.Unwrap(err), testError)
			}
			assertEqual(t, "flushed", mockProcessor.Flusher.Flushed, true)
			assertEqual(t, "mutated", mockProcessor.Mutator.Mutated, true)
			return
		}
	}
	testContextDone := func(mockProcessor mock.Processor) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			ctx, cancelFn := context.WithCancel(context.Background())
			cancelFn()
			r, _ := setupRunner(mockProcessor.Processor(), signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			})
			errc := r.Run(ctx)

			_, ok := <-r.Out()
			assertEqual(t, "out closed", ok, false)
			_, ok = <-errc
			assertEqual(t, "errc closed", ok, false)
			assertEqual(t, "processor flushed", mockProcessor.Flusher.Flushed, true)
			return
		}
	}
	t.Run("ok", testProcessor(
		context.Background(),
		mock.Processor{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
		},
	))
	t.Run("error", testProcessor(
		context.Background(),
		mock.Processor{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
			ErrorOnCall: testError,
		},
	))
	t.Run("flush error", testProcessor(
		context.Background(),
		mock.Processor{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
			Flusher: mock.Flusher{
				ErrorOnFlush: testError,
			},
		},
	))
	t.Run("context done", testContextDone(
		mock.Processor{},
	))
	t.Run("mutation error", testProcessor(
		context.Background(),
		mock.Processor{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
		},
	))
}

func TestSink(t *testing.T) {
	setupRunner := func(sinkAllocator pipe.SinkAllocatorFunc, alloc signal.Allocator) (execution.Starter, chan<- execution.Message) {
		m := mutable.Mutable()
		sink, _ := sinkAllocator(m, bufferSize, pipe.SignalProperties{Channels: channels})
		in := make(chan execution.Message, 1)
		return execution.Sink(
			m,
			in,
			signal.GetPoolAllocator(channels, bufferSize, bufferSize),
			execution.SinkFunc(sink.SinkFunc),
			execution.HookFunc(sink.StartFunc),
			execution.HookFunc(sink.FlushFunc),
		), in
	}
	testSink := func(ctx context.Context, mockSink mock.Sink) func(*testing.T) {
		return func(t *testing.T) {
			alloc := signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}

			r, in := setupRunner(mockSink.Sink(), alloc)
			errc := r.Run(ctx)
			in <- execution.Message{
				Signal:    alloc.Float64(),
				Mutations: mutable.Mutations{}.Put(mockSink.MockMutation()),
			}
			close(in)
			for err := range errc {
				assertEqual(t, "error", errors.Unwrap(err), testError)
			}
			assertEqual(t, "flushed", mockSink.Flushed, true)
			assertEqual(t, "mutated", mockSink.Mutator.Mutated, true)
			return
		}
	}
	testContextDone := func(mockSink mock.Sink) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			ctx, cancelFn := context.WithCancel(context.Background())
			cancelFn()
			alloc := signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}

			r, _ := setupRunner(mockSink.Sink(), alloc)
			_, ok := <-r.Run(ctx)
			assertEqual(t, "errc closed", ok, false)
			assertEqual(t, "flushed", mockSink.Flusher.Flushed, true)
			return
		}
	}
	t.Run("ok", testSink(
		context.Background(),
		mock.Sink{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
		},
	))
	t.Run("error", testSink(
		context.Background(),
		mock.Sink{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
			ErrorOnCall: testError,
		},
	))
	t.Run("flush error", testSink(
		context.Background(),
		mock.Sink{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
			Flusher: mock.Flusher{
				ErrorOnFlush: testError,
			},
		},
	))
	t.Run("ok", testContextDone(
		mock.Sink{},
	))
	t.Run("mutation error", testSink(
		context.Background(),
		mock.Sink{
			Mutator: mock.Mutator{
				Mutability: mutable.Mutable(),
			},
		},
	))
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
