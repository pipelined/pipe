package mock_test

import (
	"context"
	"errors"
	"io"
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
			_, fn, bus, err := pump.Pump()(p.bufferSize)
			assertError(t, nil, err)

			buf := signal.Float64Buffer(bus.NumChannels, p.bufferSize)
			for {
				if err := fn.Pump(buf); err != nil {
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
		mock.Pump{
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
		mock.Pump{
			ErrorOnCall: errTest,
		},
		params{},
	))
}

func TestProcessor(t *testing.T) {
	type params struct {
		in       signal.Float64
		expected signal.Float64
	}
	testProcessor := func(processorMock mock.Processor, p params) func(*testing.T) {
		return func(t *testing.T) {
			_, processor, _, err := processorMock.Processor()(pipe.Bus{})
			assertError(t, nil, err)

			out := signal.Float64Buffer(p.expected.NumChannels(), p.expected.Size())
			err = processor.Process(p.in, out)
			assertError(t, processorMock.ErrorOnCall, err)
			if p.expected.NumChannels() != out.NumChannels() {
				t.Fatalf("Invalid number of channels: %d expected: %d", out.NumChannels(), p.expected.NumChannels())
			}
			if p.expected.Size() != out.Size() {
				t.Fatalf("Invalid buffer size: %d expected: %d", out.Size(), p.expected.Size())
			}
		}
	}

	t.Run("1 channel", testProcessor(
		mock.Processor{},
		params{
			in:       [][]float64{{1, 1, 1, 1}},
			expected: [][]float64{{1, 1, 1, 1}},
		},
	))
	t.Run("2 channels", testProcessor(
		mock.Processor{},
		params{
			in:       [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
			expected: [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
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
		in       signal.Float64
		expected signal.Float64
	}
	testSink := func(sinkMock mock.Sink, p params) func(*testing.T) {
		return func(t *testing.T) {
			_, sink, err := sinkMock.Sink()(pipe.Bus{})
			assertError(t, nil, err)

			err = sink.Sink(p.in)
			assertError(t, sinkMock.ErrorOnCall, err)

			if p.expected.NumChannels() != sinkMock.Counter.Values.NumChannels() {
				t.Fatalf("Invalid number of channels: %d expected: %d", sinkMock.Counter.Values.NumChannels(), p.expected.NumChannels())
			}
			if p.expected.Size() != sinkMock.Counter.Values.Size() {
				t.Fatalf("Invalid buffer size: %d expected: %d", sinkMock.Counter.Values.Size(), p.expected.Size())
			}
		}
	}

	t.Run("1 channel", testSink(
		mock.Sink{
			Discard: false,
		},
		params{
			in:       [][]float64{{1, 1, 1, 1}},
			expected: [][]float64{{1, 1, 1, 1}},
		},
	))
	t.Run("2 channel", testSink(
		mock.Sink{
			Discard: false,
		},
		params{
			in:       [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
			expected: [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
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
