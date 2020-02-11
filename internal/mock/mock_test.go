package mock_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/internal/mock"
)

var errTest = errors.New("Test error")

func TestPump(t *testing.T) {
	type params struct {
		bufferSize int
		calls      int
	}
	testPump := func(options mock.PumpOptions, p params) func(*testing.T) {
		return func(t *testing.T) {
			pump, _, numChannels, err := mock.Pump(&options)(p.bufferSize)
			assertError(t, nil, err)

			buf := signal.Float64Buffer(numChannels, p.bufferSize)
			for {
				if err := pump.Pump(buf); err != nil {
					if err != io.EOF {
						assertError(t, options.ErrorOnCall, err)
					}
					break
				}
			}

			if p.calls != options.Messages {
				t.Fatalf("Invalid number of calls: %d expected: %d", options.Messages, p.calls)
			}
			if options.Limit != options.Samples {
				t.Fatalf("Invalid number of samples: %d expected: %d", options.Samples, options.Limit)
			}
		}
	}

	t.Run("3 calls", testPump(
		mock.PumpOptions{
			SampleRate:  44100,
			NumChannels: 2,
			Limit:       11,
			Value:       1,
		},
		params{
			bufferSize: 5,
			calls:      3,
		},
	))
	t.Run("500 calls", testPump(
		mock.PumpOptions{
			SampleRate:  44100,
			NumChannels: 2,
			Limit:       2500,
			Value:       2,
		},
		params{
			bufferSize: 5,
			calls:      500,
		},
	))
	t.Run("error on call", testPump(
		mock.PumpOptions{
			ErrorOnCall: errTest,
		},
		params{},
	))
}

func TestProcessor(t *testing.T) {
	type params struct {
		in         signal.Float64
		expected   signal.Float64
		bufferSize int
	}
	testProcessor := func(options mock.ProcessorOptions, p params) func(*testing.T) {
		return func(t *testing.T) {
			processor, _, _, err := mock.Processor(&options)(p.bufferSize, 0, 0)
			assertError(t, nil, err)

			out := signal.Float64Buffer(p.expected.NumChannels(), p.expected.Size())
			err = processor.Process(p.in, out)
			assertError(t, options.ErrorOnCall, err)
			if p.expected.NumChannels() != out.NumChannels() {
				t.Fatalf("Invalid number of channels: %d expected: %d", out.NumChannels(), p.expected.NumChannels())
			}
			if p.expected.Size() != out.Size() {
				t.Fatalf("Invalid buffer size: %d expected: %d", out.Size(), p.expected.Size())
			}
		}
	}

	t.Run("1 channel", testProcessor(
		mock.ProcessorOptions{},
		params{
			in:       [][]float64{{1, 1, 1, 1}},
			expected: [][]float64{{1, 1, 1, 1}},
		},
	))
	t.Run("2 channels", testProcessor(
		mock.ProcessorOptions{},
		params{
			in:       [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
			expected: [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
		},
	))
	t.Run("error on call", testProcessor(
		mock.ProcessorOptions{
			ErrorOnCall: errTest,
		},
		params{},
	))
}

func TestSink(t *testing.T) {
	type params struct {
		in         signal.Float64
		expected   signal.Float64
		bufferSize int
	}
	testSink := func(options mock.SinkOptions, p params) func(*testing.T) {
		return func(t *testing.T) {
			sink, err := mock.Sink(&options)(p.bufferSize, 0, 0)
			assertError(t, nil, err)

			err = sink.Sink(p.in)
			assertError(t, options.ErrorOnCall, err)

			if p.expected.NumChannels() != options.Counter.Values.NumChannels() {
				t.Fatalf("Invalid number of channels: %d expected: %d", options.Counter.Values.NumChannels(), p.expected.NumChannels())
			}
			if p.expected.Size() != options.Counter.Values.Size() {
				t.Fatalf("Invalid buffer size: %d expected: %d", options.Counter.Values.Size(), p.expected.Size())
			}
		}
	}

	t.Run("1 channel", testSink(
		mock.SinkOptions{
			Discard: false,
		},
		params{
			in:       [][]float64{{1, 1, 1, 1}},
			expected: [][]float64{{1, 1, 1, 1}},
		},
	))
	t.Run("2 channel", testSink(
		mock.SinkOptions{
			Discard: false,
		},
		params{
			in:       [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
			expected: [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
		},
	))
	t.Run("error on call", testSink(
		mock.SinkOptions{
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
