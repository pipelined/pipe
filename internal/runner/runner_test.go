package runner_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"pipelined.dev/pipe"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutability"
	"pipelined.dev/pipe/pool"
)

var testError = errors.New("test runner error")

const (
	bufferSize = 1024
	channels   = 1
)

func TestPump(t *testing.T) {
	setupPump := func(pumpMaker pipe.PumpMaker) runner.Pump {
		pump, bus, _ := pumpMaker(bufferSize)
		return runner.Pump{
			Mutability: pump.Mutability,
			Output: pool.Get(signal.Allocator{
				Channels: bus.Channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}),
			Fn:    pump.Pump,
			Flush: runner.Flush(pump.Flush),
			Meter: metric.Meter(pump, bus.SampleRate),
		}
	}
	assertPump := func(t *testing.T, mockPump *mock.Pump, out <-chan runner.Message, errs <-chan error) {
		t.Helper()
		received := 0
		for buf := range out {
			received = received + buf.Signal.Length()
		}
		for err := range errs {
			assertEqual(t, "error", errors.Unwrap(err), testError)
		}
		assertEqual(t, "pump samples", received, mockPump.Limit)
		assertEqual(t, "pump flushed", mockPump.Flushed, true)
		return
	}
	testPump := func(ctx context.Context, mockPump mock.Pump) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			r := setupPump(mockPump.Pump())
			mutations := make(chan mutability.Mutations)
			out, errs := r.Run(ctx, mutations)
			assertPump(t, &mockPump, out, errs)
		}
	}
	testContextDone := func(mockPump mock.Pump) func(*testing.T) {
		t.Helper()
		cancelCtx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		return testPump(cancelCtx, mockPump)
	}

	testMutationError := func(ctx context.Context, mockPump mock.Pump) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			r := setupPump(mockPump.Pump())
			mutations := make(chan mutability.Mutations, 1)
			mutations <- mutability.Mutations{}.Put(mockPump.MockMutation())
			out, errs := r.Run(ctx, mutations)
			assertPump(t, &mockPump, out, errs)
		}
	}

	t.Run("ok", testPump(
		context.Background(),
		mock.Pump{
			Channels: 1,
			Limit:    10*bufferSize + 1,
		},
	))
	t.Run("error", testPump(
		context.Background(),
		mock.Pump{
			ErrorOnCall: testError,
			Channels:    1,
			Limit:       0,
		},
	))
	t.Run("flush error", testPump(
		context.Background(),
		mock.Pump{
			Flusher: mock.Flusher{
				ErrorOnFlush: testError,
			},
			Channels: 1,
			Limit:    10 * bufferSize,
		},
	))

	t.Run("context done", testContextDone(
		mock.Pump{
			Channels: 1,
			Limit:    0,
		},
	))
	t.Run("mutation error", testMutationError(
		context.Background(),
		mock.Pump{
			ErrorOnMutation: testError,
			Mutability:      mutability.New(),
			Channels:        1,
			Limit:           0,
		},
	))
}

func TestProcessor(t *testing.T) {
	setupRunner := func(processorMaker pipe.ProcessorMaker, alloc signal.Allocator) runner.Processor {
		processor, bus, _ := processorMaker(bufferSize, pipe.Bus{Channels: channels})
		return runner.Processor{
			Mutability: processor.Mutability,
			Output: pool.Get(signal.Allocator{
				Channels: bus.Channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}),
			Fn:    processor.Process,
			Flush: runner.Flush(processor.Flush),
			Meter: metric.Meter(processor, bus.SampleRate),
		}
	}
	testProcessor := func(ctx context.Context, mockProcessor mock.Processor) func(*testing.T) {
		return func(t *testing.T) {
			alloc := signal.Allocator{
				Channels: channels,
				Length:   bufferSize,
				Capacity: bufferSize,
			}
			in := make(chan runner.Message)

			r := setupRunner(mockProcessor.Processor(), alloc)
			out, errc := r.Run(ctx, in)

			select {
			case <-ctx.Done():
			default:
				in <- runner.Message{Signal: alloc.Float64()}
				close(in)
			}
			for msg := range out {
				assertEqual(t, "processed samples", msg.Signal.Length(), alloc.Length)
			}
			for err := range errc {
				assertEqual(t, "error", errors.Unwrap(err), testError)
			}
			assertEqual(t, "pump flushed", mockProcessor.Flushed, true)
			return
		}
	}

	testContextDone := func(mockProcessor mock.Processor) func(*testing.T) {
		t.Helper()
		cancelCtx, cancelFn := context.WithCancel(context.Background())
		cancelFn()
		return testProcessor(cancelCtx, mockProcessor)
	}
	t.Run("ok", testProcessor(
		context.Background(),
		mock.Processor{},
	))
	t.Run("error", testProcessor(
		context.Background(),
		mock.Processor{
			ErrorOnCall: testError,
		},
	))
	t.Run("flush error", testProcessor(
		context.Background(),
		mock.Processor{
			Flusher: mock.Flusher{
				ErrorOnFlush: testError,
			},
		},
	))
	t.Run("context doe", testContextDone(
		mock.Processor{},
	))
}

// func TestProcessorRunner(t *testing.T) {
// 	tests := []struct {
// 		messages        int
// 		cancelOnReceive bool
// 		cancelOnSend    bool
// 		processor       *mock.Processor
// 	}{
// 		{
// 			messages:  10,
// 			processor: &mock.Processor{},
// 		},
// 		{
// 			processor: &mock.Processor{
// 				ErrorOnCall: testError,
// 			},
// 		},
// 		{
// 			cancelOnReceive: true,
// 			processor:       &mock.Processor{},
// 		},
// 		// This test case cannot guarantee coverage because buffered out channel is used.
// 		// {
// 		// 	cancelOnSend: true,
// 		// 	processor:    &mock.Processor{},
// 		// },
// 		{
// 			processor: &mock.Processor{
// 				Hooks: mock.Hooks{
// 					ErrorOnReset: testError,
// 				},
// 			},
// 		},
// 	}
// 	sampleRate := signal.SampleRate(44100)
// 	numChannels := 1
// 	for _, c := range tests {
// 		fn, _ := c.processor.Process(pipeID, sampleRate, numChannels)
// 		r := runner.Processor{
// 			Fn:    fn,
// 			Meter: metric.Meter(c.processor, signal.SampleRate(sampleRate)),
// 			Hooks: pipe.BindHooks(c.processor),
// 		}

// 		cancel := make(chan struct{})
// 		in := make(chan runner.Message)
// 		out, errs := r.Run(pipeID, componentID, cancel, in)
// 		assert.NotNil(t, out)
// 		assert.NotNil(t, errs)

// 		switch {
// 		case c.cancelOnReceive:
// 			close(cancel)
// 		case c.cancelOnSend:
// 			in <- runner.Message{
// 				PipeID: pipeID,
// 			}
// 			close(cancel)
// 		case c.processor.ErrorOnCall != nil:
// 			in <- runner.Message{
// 				PipeID: pipeID,
// 			}
// 			err := <-errs
// 			assert.Equal(t, c.processor.ErrorOnCall, errors.Unwrap(err))
// 		case c.processor.ErrorOnReset != nil:
// 			err := <-errs
// 			assert.Equal(t, c.processor.ErrorOnReset, errors.Unwrap(err))
// 		default:
// 			for i := 0; i <= c.messages; i++ {
// 				in <- runner.Message{
// 					PipeID: pipeID,
// 				}
// 				<-out
// 			}
// 			close(in)
// 		}

// 		pipe.Wait(errs)

// 		assert.True(t, c.processor.Resetted)
// 		if c.processor.ErrorOnReset != nil {
// 			assert.False(t, c.processor.Flushed)
// 		} else {
// 			assert.True(t, c.processor.Flushed)
// 		}

// 		switch {
// 		case c.cancelOnReceive || c.cancelOnSend:
// 			assert.True(t, c.processor.Interrupted)
// 		default:
// 			assert.False(t, c.processor.Interrupted)
// 		}
// 	}
// }

// func TestSinkRunner(t *testing.T) {
// 	tests := []struct {
// 		messages        int
// 		cancelOnReceive bool
// 		sink            *mock.Sink
// 	}{
// 		{
// 			messages: 10,
// 			sink:     &mock.Sink{},
// 		},
// 		{
// 			sink: &mock.Sink{
// 				ErrorOnCall: testError,
// 			},
// 		},
// 		{
// 			cancelOnReceive: true,
// 			sink:            &mock.Sink{},
// 		},
// 		{
// 			sink: &mock.Sink{
// 				Hooks: mock.Hooks{
// 					ErrorOnReset: testError,
// 				},
// 			},
// 		},
// 	}

// 	sampleRate := signal.SampleRate(44100)
// 	numChannels := 1
// 	for _, c := range tests {
// 		fn, _ := c.sink.Sink(pipeID, sampleRate, numChannels)

// 		r := runner.Sink{
// 			Fn:    fn,
// 			Meter: metric.Meter(c.sink, signal.SampleRate(sampleRate)),
// 			Hooks: pipe.BindHooks(c.sink),
// 		}

// 		cancel := make(chan struct{})
// 		in := make(chan runner.Message)
// 		errs := r.Run(noOpPool{}, pipeID, componentID, cancel, in)
// 		assert.NotNil(t, errs)

// 		switch {
// 		case c.cancelOnReceive:
// 			close(cancel)
// 		case c.sink.ErrorOnCall != nil:
// 			in <- runner.Message{
// 				PipeID: pipeID,
// 			}
// 			err := <-errs
// 			assert.Equal(t, c.sink.ErrorOnCall, errors.Unwrap(err))
// 		case c.sink.ErrorOnReset != nil:
// 			err := <-errs
// 			assert.Equal(t, c.sink.ErrorOnReset, errors.Unwrap(err))
// 		default:
// 			for i := 0; i <= c.messages; i++ {
// 				in <- runner.Message{
// 					SinkRefs: 1,
// 					PipeID:   pipeID,
// 				}
// 			}
// 			close(in)
// 		}

// 		pipe.Wait(errs)

// 		assert.True(t, c.sink.Resetted)
// 		if c.sink.ErrorOnReset != nil {
// 			assert.False(t, c.sink.Flushed)
// 		} else {
// 			assert.True(t, c.sink.Flushed)
// 		}

// 		switch {
// 		case c.cancelOnReceive:
// 			assert.True(t, c.sink.Interrupted)
// 		default:
// 			assert.False(t, c.sink.Interrupted)
// 		}
// 	}
// }

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
