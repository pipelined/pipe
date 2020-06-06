package mock_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/mock"
	"pipelined.dev/pipe/mutability"
)

var errTest = errors.New("Test error")

func TestPump(t *testing.T) {
	type params struct {
		bufferSize int
		calls      int
	}
	testPump := func(pump mock.Pump, test params) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			fn, bus, _ := pump.Pump()(test.bufferSize)
			buf := signal.Allocator{
				Channels: bus.Channels,
				Length:   test.bufferSize,
				Capacity: test.bufferSize,
			}.Float64()
			for {
				if _, err := fn.Pump(buf); err != nil {
					if err != io.EOF {
						assertEqual(t, "error", err, pump.ErrorOnCall)
					}
					break
				}
			}

			assertEqual(t, "calls", pump.Messages, test.calls)
			assertEqual(t, "samples", pump.Samples, pump.Limit)
		}
	}

	testMutations := func(test params) func(*testing.T) {
		mockPump := &mock.Pump{
			Mutator: mock.Mutator{
				Mutability: mutability.Mutable(),
			},
			Limit:    test.bufferSize,
			Channels: 1,
		}
		pump, bus, _ := mockPump.Pump()(10)
		pump.Pump(signal.Allocator{
			Channels: bus.Channels,
			Length:   test.bufferSize,
			Capacity: test.bufferSize,
		}.Float64())
		return func(t *testing.T) {
			t.Helper()
			assertEqual(t, "counter before reset", mockPump.Counter.Samples, test.bufferSize)
			mockPump.Reset().Apply()
			assertEqual(t, "counter before after", mockPump.Counter.Samples, 0)
			mockPump.MockMutation().Apply()
		}
	}

	t.Run("3 calls", testPump(
		mock.Pump{
			SampleRate: 44100,
			Channels:   2,
			Limit:      11,
			Value:      1,
		},
		params{
			bufferSize: 5,
			calls:      3,
		},
	))
	t.Run("500 calls", testPump(
		mock.Pump{
			SampleRate: 44100,
			Channels:   2,
			Limit:      2500,
			Value:      2,
		},
		params{
			bufferSize: 5,
			calls:      500,
		},
	))
	t.Run("error on call", testPump(
		mock.Pump{
			ErrorOnCall: errTest,
		},
		params{},
	))
	t.Run("mutations", testMutations(params{
		bufferSize: 10,
	}))
}

func TestProcessor(t *testing.T) {
	type params struct {
		in       []float64
		expected []float64
	}
	testProcessor := func(processorMock mock.Processor, p params) func(*testing.T) {
		return func(t *testing.T) {
			processor, _, _ := processorMock.Processor()(0, pipe.Bus{})

			alloc := signal.Allocator{
				Channels: 1,
				Capacity: len(p.in),
				Length:   len(p.in),
			}
			in, out := alloc.Float64(), alloc.Float64()
			signal.WriteFloat64(p.in, in)

			err := processor.Process(in, out)
			assertEqual(t, "error", err, processorMock.ErrorOnCall)
			if err != nil {
				return
			}
			result := make([]float64, len(p.expected))
			signal.ReadFloat64(out, result)
			assertEqual(t, "slice", result, p.expected)
		}
	}

	t.Run("1 channel", testProcessor(
		mock.Processor{},
		params{
			in:       []float64{1, 1, 1, 1},
			expected: []float64{1, 1, 1, 1},
		},
	))
	t.Run("2 channels", testProcessor(
		mock.Processor{},
		params{
			in:       []float64{1, 1, 1, 1, 2, 2, 2, 2},
			expected: []float64{1, 1, 1, 1, 2, 2, 2, 2},
		},
	))
	t.Run("error on call", testProcessor(
		mock.Processor{
			ErrorOnCall: errTest,
		},
		params{},
	))
}

func TestSink(t *testing.T) {
	type params struct {
		in       []float64
		expected []float64
	}
	testSink := func(sinkMock mock.Sink, p params) func(*testing.T) {
		return func(t *testing.T) {
			sink, _ := sinkMock.Sink()(0, pipe.Bus{Channels: 1})

			alloc := signal.Allocator{
				Channels: 1,
				Capacity: len(p.in),
				Length:   len(p.in),
			}
			in := alloc.Float64()
			signal.WriteFloat64(p.in, in)

			err := sink.Sink(in)
			assertEqual(t, "error", err, sinkMock.ErrorOnCall)
			if err != nil {
				return
			}

			result := make([]float64, len(p.expected))
			signal.ReadFloat64(sinkMock.Counter.Values, result)
			assertEqual(t, "slice", result, p.expected)
		}
	}

	t.Run("1 channel", testSink(
		mock.Sink{
			Discard: false,
		},
		params{
			in:       []float64{1, 1, 1, 1},
			expected: []float64{1, 1, 1, 1},
		},
	))
	t.Run("2 channel", testSink(
		mock.Sink{
			Discard: false,
		},
		params{
			in:       []float64{1, 1, 1, 1, 2, 2, 2, 2},
			expected: []float64{1, 1, 1, 1, 2, 2, 2, 2},
		},
	))
	t.Run("error on call", testSink(
		mock.Sink{
			ErrorOnCall: errTest,
		},
		params{},
	))
}

func TestHooks(t *testing.T) {
	testFlush := func(f *mock.Flusher, expected error) func(*testing.T) {
		return func(t *testing.T) {
			err := f.Flush(context.Background())
			assertEqual(t, "error", err, expected)
		}
	}

	t.Run("flush nil", testFlush(&mock.Flusher{}, nil))
	t.Run("flush err", testFlush(&mock.Flusher{ErrorOnFlush: errTest}, errTest))
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
