package runner_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/pipe/mutability"

	"pipelined.dev/signal"
)

var testError = errors.New("test runner error")

const (
	bufferSize = 1024
	channels   = 1
)

func TestSource(t *testing.T) {
	setupSource := func(sourceAllocator pipe.SourceAllocatorFunc) runner.Source {
		source, props, _ := sourceAllocator(context.Background(), bufferSize)
		return runner.Source{
			Mutability: source.Mutability,
			OutPool:    signal.GetPoolAllocator(props.Channels, bufferSize, bufferSize),
			Fn:         source.SourceFunc,
			Flush:      runner.Flush(source.FlushFunc),
		}
	}
	assertSource := func(t *testing.T, mockSource *mock.Source, out <-chan runner.Message, errs <-chan error) {
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
			r := setupSource(mockSource.Source())
			mutations := make(chan mutability.Mutations)
			out, errs := r.Run(ctx, mutations)
			assertSource(t, &mockSource, out, errs)
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
			r := setupSource(mockSource.Source())
			mutations := make(chan mutability.Mutations, 1)
			mutations <- mutability.Mutations{}.Put(mockSource.MockMutation())
			out, errs := r.Run(ctx, mutations)
			assertSource(t, &mockSource, out, errs)
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
				Mutability:      mutability.Mutable(),
				ErrorOnMutation: testError,
			},
			Channels: 1,
			Limit:    0,
		},
	))
}

func TestProcessor(t *testing.T) {
	setupRunner := func(processorAllocator pipe.ProcessorAllocatorFunc, alloc signal.Allocator) runner.Processor {
		processor, props, _ := processorAllocator(context.Background(), bufferSize, pipe.SignalProperties{Channels: channels})
		return runner.Processor{
			Mutability: processor.Mutability,
			InPool:     signal.GetPoolAllocator(props.Channels, bufferSize, bufferSize),
			OutPool:    signal.GetPoolAllocator(props.Channels, bufferSize, bufferSize),
			Fn:         processor.ProcessFunc,
			Flush:      runner.Flush(processor.FlushFunc),
		}
	}
	testProcessor := func(ctx context.Context, mockProcessor mock.Processor) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			alloc := signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}
			in := make(chan runner.Message, 1)
			r := setupRunner(mockProcessor.Processor(), alloc)
			out, errc := r.Run(ctx, in)

			// test mutations only for mutable
			in <- runner.Message{
				Signal:    alloc.Float64(),
				Mutations: mutability.Mutations{}.Put(mockProcessor.MockMutation()),
			}
			close(in)
			for msg := range out {
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
			r := setupRunner(mockProcessor.Processor(), signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			})
			out, errc := r.Run(ctx, make(chan runner.Message))

			_, ok := <-out
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
				Mutability: mutability.Mutable(),
			},
		},
	))
	t.Run("error", testProcessor(
		context.Background(),
		mock.Processor{
			Mutator: mock.Mutator{
				Mutability: mutability.Mutable(),
			},
			ErrorOnCall: testError,
		},
	))
	t.Run("flush error", testProcessor(
		context.Background(),
		mock.Processor{
			Mutator: mock.Mutator{
				Mutability: mutability.Mutable(),
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
				Mutability:      mutability.Mutable(),
				ErrorOnMutation: testError,
			},
		},
	))
}

func TestSink(t *testing.T) {
	setupRunner := func(sinkAllocator pipe.SinkAllocatorFunc, alloc signal.Allocator) runner.Sink {
		sink, _ := sinkAllocator(context.Background(), bufferSize, pipe.SignalProperties{Channels: channels})
		return runner.Sink{
			Mutability: sink.Mutability,
			InPool:     signal.GetPoolAllocator(channels, bufferSize, bufferSize),
			Fn:         sink.SinkFunc,
			Flush:      runner.Flush(sink.FlushFunc),
		}
	}
	testSink := func(ctx context.Context, mockSink mock.Sink) func(*testing.T) {
		return func(t *testing.T) {
			alloc := signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}
			in := make(chan runner.Message, 1)

			r := setupRunner(mockSink.Sink(), alloc)
			errc := r.Run(ctx, in)
			in <- runner.Message{
				Signal:    alloc.Float64(),
				Mutations: mutability.Mutations{}.Put(mockSink.MockMutation()),
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

			errc := setupRunner(mockSink.Sink(), alloc).Run(ctx, make(chan runner.Message))
			_, ok := <-errc
			assertEqual(t, "errc closed", ok, false)
			assertEqual(t, "flushed", mockSink.Flusher.Flushed, true)
			return
		}
	}
	t.Run("ok", testSink(
		context.Background(),
		mock.Sink{
			Mutator: mock.Mutator{
				Mutability: mutability.Mutable(),
			},
		},
	))
	t.Run("error", testSink(
		context.Background(),
		mock.Sink{
			Mutator: mock.Mutator{
				Mutability: mutability.Mutable(),
			},
			ErrorOnCall: testError,
		},
	))
	t.Run("flush error", testSink(
		context.Background(),
		mock.Sink{
			Mutator: mock.Mutator{
				Mutability: mutability.Mutable(),
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
				Mutability:      mutability.Mutable(),
				ErrorOnMutation: testError,
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
