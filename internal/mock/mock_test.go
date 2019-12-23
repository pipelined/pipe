package mock_test

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
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
			counter := &mock.Counter{}
			pump, _, numChannels, err := mock.Pump(counter, nil, options)()
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

			assert.Equal(t, p.calls, counter.Messages)
			assert.Equal(t, options.Limit, counter.Samples)
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
		in       signal.Float64
		expected signal.Float64
	}
	testProcessor := func(options mock.ProcessorOptions, p params) func(*testing.T) {
		return func(t *testing.T) {
			counter := &mock.Counter{}
			processor, _, _, err := mock.Processor(counter, nil, options)(0, 0)
			assertError(t, nil, err)

			out := signal.Float64Buffer(p.expected.NumChannels(), p.expected.Size())
			err = processor.Process(p.in, out)
			assertError(t, options.ErrorOnCall, err)
			assert.Equal(t, p.expected.NumChannels(), out.NumChannels())
			assert.Equal(t, p.expected.Size(), out.Size())
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
		in       signal.Float64
		expected signal.Float64
	}
	testSink := func(options mock.SinkOptions, p params) func(*testing.T) {
		return func(t *testing.T) {
			counter := &mock.Counter{}
			sink, err := mock.Sink(counter, nil, options)(0, 0)
			assertError(t, nil, err)

			err = sink.Sink(p.in)
			assertError(t, options.ErrorOnCall, err)
			assert.Equal(t, p.expected.NumChannels(), counter.Values.NumChannels())
			assert.Equal(t, p.expected.Size(), counter.Values.Size())
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
	testReset := func(h *mock.Hooks, expected error) func(*testing.T) {
		return func(t *testing.T) {
			err := h.Reset()
			assert.Equal(t, expected, err)
		}
	}
	testFlush := func(h *mock.Hooks, expected error) func(*testing.T) {
		return func(t *testing.T) {
			err := h.Flush()
			assert.Equal(t, expected, err)
		}
	}
	testInterrupt := func(h *mock.Hooks, expected error) func(*testing.T) {
		return func(t *testing.T) {
			err := h.Interrupt()
			assert.Equal(t, expected, err)
		}
	}

	t.Run("flush nil", testFlush(&mock.Hooks{}, nil))
	t.Run("reset nil", testReset(&mock.Hooks{}, nil))
	t.Run("interrupt nil", testInterrupt(&mock.Hooks{}, nil))

	t.Run("flush", testFlush(&mock.Hooks{ErrorOnFlush: errTest}, errTest))
	t.Run("reset", testReset(&mock.Hooks{ErrorOnReset: errTest}, errTest))
	t.Run("interrupt", testInterrupt(&mock.Hooks{ErrorOnInterrupt: errTest}, errTest))
}

func assertError(t *testing.T, expected, err error) {
	if err != expected {
		t.Fatalf("Unexpected error: %v expected: %v", err, expected)
	}
}
