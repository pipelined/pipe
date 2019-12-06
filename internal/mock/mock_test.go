package mock_test

import (
	"errors"
	"fmt"
	"io"
	"math"
	"testing"
	"time"

	"pipelined.dev/signal"
	"github.com/stretchr/testify/assert"

	"pipelined.dev/pipe/internal/mock"
)

var (
	testError = errors.New("Test error")
)

func TestPump(t *testing.T) {
	tests := []struct {
		sampleRate  signal.SampleRate
		numChannels int
		limit       int
		interval    time.Duration
		value       float64
		bufferSize  int
		errorOnCall error
	}{
		{
			sampleRate:  44100,
			numChannels: 2,
			limit:       125,
			value:       1,
			bufferSize:  5,
		},
		{
			sampleRate:  44100,
			numChannels: 2,
			limit:       2500,
			value:       2,
			bufferSize:  5,
		},
		{
			sampleRate:  44100,
			numChannels: 2,
			limit:       125,
			value:       1,
			bufferSize:  3,
		},
		{
			errorOnCall: testError,
		},
	}

	for _, test := range tests {
		pump := &mock.Pump{
			SampleRate:  test.sampleRate,
			NumChannels: test.numChannels,
			Limit:       test.limit,
			Interval:    test.interval,
			ErrorOnCall: test.errorOnCall,
		}
		err := pump.Reset("")
		assert.Nil(t, err)
		fn, sampleRate, numChannels, err := pump.Pump("")
		assert.NotNil(t, fn)
		assert.Equal(t, test.sampleRate, sampleRate)
		assert.Equal(t, test.numChannels, numChannels)
		assert.Nil(t, err)

		buf := signal.Float64Buffer(test.numChannels, test.bufferSize)

		// test negative flow
		if test.errorOnCall != nil {
			err := fn(buf)
			assert.Equal(t, test.errorOnCall, err)
			continue
		}

		// positive flow
		calls := int(math.Floor(float64(test.limit) / float64(test.bufferSize)))
		remainder := test.limit % test.bufferSize
		for i := 0; i < calls; i++ {
			err := fn(buf)
			assert.NotNil(t, buf)
			assert.Nil(t, err)
			assert.Equal(t, test.bufferSize, signal.Float64(buf).Size())
			assert.Equal(t, test.numChannels, signal.Float64(buf).NumChannels())
		}
		if remainder != 0 {
			calls++
			err := fn(buf)
			assert.NoError(t, err)
			assert.Equal(t, remainder, signal.Float64(buf).Size())
		}
		err = fn(buf)
		assert.Equal(t, io.EOF, err)

		// test params
		paramFn := pump.IntervalParam(test.interval)
		assert.NotNil(t, paramFn)
		paramFn()
		paramFn = pump.LimitParam(test.limit)
		assert.NotNil(t, paramFn)
		paramFn()
		paramFn = pump.ValueParam(test.value)
		assert.NotNil(t, paramFn)
		paramFn()
		messages, samples := pump.Count()
		assert.Equal(t, calls, messages)
		assert.Equal(t, test.limit, samples)
	}
}

func TestProcessor(t *testing.T) {
	tests := []struct {
		sampleRate  signal.SampleRate
		numChannels int
		bufferSize  int
		buf         signal.Float64
		expected    [][]float64
		errorOnCall error
	}{
		{
			sampleRate:  44100,
			numChannels: 2,
			bufferSize:  5,
			buf:         [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
			expected:    [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
		},
		{
			sampleRate:  44100,
			numChannels: 2,
			bufferSize:  5,
			buf:         [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
			expected:    [][]float64{{1, 1, 1, 1}, {2, 2, 2, 2}},
		},
		{
			errorOnCall: testError,
		},
	}

	for _, test := range tests {
		processor := &mock.Processor{
			ErrorOnCall: test.errorOnCall,
		}
		err := processor.Reset("")
		assert.Nil(t, err)
		fn, err := processor.Process("", test.sampleRate, test.numChannels)
		assert.NotNil(t, fn)
		assert.Nil(t, err)

		// test negative flow
		if test.errorOnCall != nil {
			err := fn(test.buf)
			assert.Equal(t, test.errorOnCall, err)
			continue
		}

		// positive flow
		err = fn(test.buf)
		assert.Nil(t, err)
		assert.Equal(t, signal.Float64(test.expected).Size(), test.buf.Size())
		assert.Equal(t, signal.Float64(test.expected).NumChannels(), test.buf.NumChannels())

		messages, samples := processor.Count()
		assert.Equal(t, 1, messages)
		assert.Equal(t, signal.Float64(test.expected).Size(), samples)
	}
}

func TestSink(t *testing.T) {
	tests := []struct {
		sink     *mock.Sink
		buf      [][]float64
		expected [][]float64
	}{
		{
			sink:     &mock.Sink{},
			buf:      [][]float64{{1, 1, 1, 1}},
			expected: [][]float64{{1, 1, 1, 1}},
		},
		{
			sink: &mock.Sink{
				ErrorOnCall: testError,
			},
		},
	}

	for _, test := range tests {
		sink := test.sink
		err := sink.Reset("")
		assert.Nil(t, err)
		fn, err := sink.Sink("", 0, 0)
		assert.Nil(t, err)
		err = fn(test.buf)
		assert.Equal(t, test.sink.ErrorOnCall, err)

		// break negative path
		if test.sink.ErrorOnCall != nil {
			continue
		}
		result := sink.Buffer()
		assert.Equal(t, len(test.expected), len(result))
		for i := range result {
			assert.Equal(t, len(test.expected[i]), len(result[i]))
			for j := range result[i] {
				assert.Equal(t, test.expected[i][j], result[i][j])
			}
		}
		messages, samples := sink.Count()
		assert.Equal(t, 1, messages)
		assert.Equal(t, signal.Float64(test.expected).Size(), samples)
	}
}

type (
	component interface {
		Reset(string) error
		Interrupt(string) error
		Flush(string) error
	}
)

func TestHooks(t *testing.T) {
	pipeID := "testPipeID"
	errMock := fmt.Errorf("Test error")
	tests := []struct {
		component
		expectedOnReset     error
		expectedOnInterrupt error
		expectedOnFlush     error
	}{
		// Pump
		{
			component: &mock.Pump{},
		},
		{
			component: &mock.Pump{
				Hooks: mock.Hooks{
					ErrorOnReset: errMock,
				},
			},
			expectedOnReset: errMock,
		},
		{
			component: &mock.Pump{
				Hooks: mock.Hooks{
					ErrorOnInterrupt: errMock,
				},
			},
			expectedOnInterrupt: errMock,
		},
		{
			component: &mock.Pump{
				Hooks: mock.Hooks{
					ErrorOnFlush: errMock,
				},
			},
			expectedOnFlush: errMock,
		},
		// Processor
		{
			component: &mock.Processor{},
		},
		{
			component: &mock.Processor{
				Hooks: mock.Hooks{
					ErrorOnReset: errMock,
				},
			},
			expectedOnReset: errMock,
		},
		{
			component: &mock.Processor{
				Hooks: mock.Hooks{
					ErrorOnInterrupt: errMock,
				},
			},
			expectedOnInterrupt: errMock,
		},
		{
			component: &mock.Processor{
				Hooks: mock.Hooks{
					ErrorOnFlush: errMock,
				},
			},
			expectedOnFlush: errMock,
		},
		// Processor
		{
			component: &mock.Sink{},
		},
		{
			component: &mock.Sink{
				Hooks: mock.Hooks{
					ErrorOnReset: errMock,
				},
			},
			expectedOnReset: errMock,
		},
		{
			component: &mock.Sink{
				Hooks: mock.Hooks{
					ErrorOnInterrupt: errMock,
				},
			},
			expectedOnInterrupt: errMock,
		},
		{
			component: &mock.Sink{
				Hooks: mock.Hooks{
					ErrorOnFlush: errMock,
				},
			},
			expectedOnFlush: errMock,
		},
	}

	for _, test := range tests {
		var err error
		err = test.component.Reset(pipeID)
		assert.Equal(t, test.expectedOnReset, err)

		err = test.component.Interrupt(pipeID)
		assert.Equal(t, test.expectedOnInterrupt, err)

		err = test.component.Flush(pipeID)
		assert.Equal(t, test.expectedOnFlush, err)
	}
}
