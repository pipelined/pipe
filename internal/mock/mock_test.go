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
)

var errTest = errors.New("Test error")

func TestPump(t *testing.T) {
	type params struct {
		bufferSize int
		calls      int
	}
	testPump := func(pump mock.Pump, p params) func(*testing.T) {
		return func(t *testing.T) {
			fn, bus, err := pump.Pump()(p.bufferSize)
			assertError(t, nil, err)
			buf := signal.Allocator{
				Channels: bus.Channels,
				Length:   p.bufferSize,
				Capacity: p.bufferSize,
			}.Float64()
			for {
				if _, err := fn.Pump(buf); err != nil {
					if err != io.EOF {
						assertError(t, pump.ErrorOnCall, err)
					}
					break
				}
			}

			if p.calls != pump.Messages {
				t.Fatalf("Invalid number of calls: %d expected: %d", pump.Messages, p.calls)
			}
			if pump.Limit != pump.Samples {
				t.Fatalf("Invalid number of samples: %d expected: %d", pump.Samples, pump.Limit)
			}
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
}

func TestProcessor(t *testing.T) {
	type params struct {
		in       []float64
		expected []float64
	}
	testProcessor := func(processorMock mock.Processor, p params) func(*testing.T) {
		return func(t *testing.T) {
			processor, _, err := processorMock.Processor()(0, pipe.Bus{})
			assertError(t, nil, err)

			alloc := signal.Allocator{
				Channels: 1,
				Capacity: len(p.in),
				Length:   len(p.in),
			}
			in, out := alloc.Float64(), alloc.Float64()
			signal.WriteFloat64(p.in, in)

			err = processor.Process(in, out)
			assertError(t, processorMock.ErrorOnCall, err)
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
			sink, err := sinkMock.Sink()(0, pipe.Bus{Channels: 1})
			assertError(t, nil, err)

			alloc := signal.Allocator{
				Channels: 1,
				Capacity: len(p.in),
				Length:   len(p.in),
			}
			in := alloc.Float64()
			signal.WriteFloat64(p.in, in)

			err = sink.Sink(in)
			assertError(t, sinkMock.ErrorOnCall, err)
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
			assertError(t, expected, err)
		}
	}

	t.Run("flush nil", testFlush(&mock.Flusher{}, nil))
	t.Run("flush err", testFlush(&mock.Flusher{ErrorOnFlush: errTest}, errTest))
}

func assertError(t *testing.T, expected, err error) {
	if err != expected {
		t.Fatalf("Unexpected error: %v expected: %v", err, expected)
	}
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
