package mock_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"pipelined.dev/signal"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/pipe/mutability"
)

var errTest = errors.New("Test error")

func TestSource(t *testing.T) {
	type params struct {
		bufferSize int
		calls      int
	}
	testSource := func(source mock.Source, test params) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			src, _ := source.Source()(test.bufferSize)
			buf := signal.Allocator{
				Channels: src.Output.Channels,
				Length:   test.bufferSize,
				Capacity: test.bufferSize,
			}.Float64()
			for {
				if _, err := src.SourceFunc(buf); err != nil {
					if err != io.EOF {
						assertEqual(t, "error", err, source.ErrorOnCall)
					}
					break
				}
			}

			assertEqual(t, "calls", source.Messages, test.calls)
			assertEqual(t, "samples", source.Samples, source.Limit)
		}
	}

	testMutations := func(test params) func(*testing.T) {
		mockSource := &mock.Source{
			Mutator: mock.Mutator{
				Mutability: mutability.Mutable(),
			},
			Limit:    test.bufferSize,
			Channels: 1,
		}
		source, _ := mockSource.Source()(10)
		source.SourceFunc(signal.Allocator{
			Channels: source.Output.Channels,
			Length:   test.bufferSize,
			Capacity: test.bufferSize,
		}.Float64())
		return func(t *testing.T) {
			t.Helper()
			assertEqual(t, "counter before reset", mockSource.Counter.Samples, test.bufferSize)
			mockSource.Reset().Apply()
			assertEqual(t, "counter before after", mockSource.Counter.Samples, 0)
			mockSource.MockMutation().Apply()
		}
	}

	t.Run("3 calls", testSource(
		mock.Source{
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
	t.Run("500 calls", testSource(
		mock.Source{
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
	t.Run("error on call", testSource(
		mock.Source{
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
			processor, _ := processorMock.Processor()(0, pipe.SignalProperties{})

			alloc := signal.Allocator{
				Channels: 1,
				Capacity: len(p.in),
				Length:   len(p.in),
			}
			in, out := alloc.Float64(), alloc.Float64()
			signal.WriteFloat64(p.in, in)

			err := processor.ProcessFunc(in, out)
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
			sink, _ := sinkMock.Sink()(0, pipe.SignalProperties{Channels: 1})

			alloc := signal.Allocator{
				Channels: 1,
				Capacity: len(p.in),
				Length:   len(p.in),
			}
			in := alloc.Float64()
			signal.WriteFloat64(p.in, in)

			err := sink.SinkFunc(in)
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
