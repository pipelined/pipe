package mock

import (
	"io"
	"time"

	"github.com/pipelined/phono/signal"
)

// Param types
type (
	// Limit messages
	Limit int
)

// Pump mocks a pipe.Pump interface.
type Pump struct {
	counter
	Interval time.Duration
	Limit
	Value       float64
	NumChannels int
	SampleRate  int
}

// Sink mocks up a pipe.Sink interface.
// Buffer is not thread-safe, so should not be checked while pipe is running.
type Sink struct {
	counter
	buffer signal.Float64
}

// Processor mocks a pipe.Processor interface.
type Processor struct {
	counter
}

// IntervalParam pushes new interval value for pump.
func (m *Pump) IntervalParam(i time.Duration) func() {
	return func() {
		m.Interval = i
	}
}

// LimitParam pushes new limit value for pump.
func (m *Pump) LimitParam(l Limit) func() {
	return func() {
		m.Limit = l
	}
}

// ValueParam pushes new signal value for pump.
func (m *Pump) ValueParam(v float64) func() {
	return func() {
		m.Value = v
	}
}

// NumChannelsParam pushes new number of channels for pump
func (m *Pump) NumChannelsParam(numChannels int) func() {
	return func() {
		m.NumChannels = numChannels
	}
}

// Pump returns new buffer for pipe.
func (m *Pump) Pump(sourceID string, bufferSize int) (func() ([][]float64, error), int, int, error) {
	return func() ([][]float64, error) {
		if Limit(m.Messages()) >= m.Limit {
			return nil, io.EOF
		}
		time.Sleep(m.Interval)

		b := make([][]float64, m.NumChannels)
		for i := range b {
			b[i] = make([]float64, bufferSize)
			for j := range b[i] {
				b[i][j] = m.Value
			}
		}
		m.counter.Advance(b)
		return b, nil
	}, m.SampleRate, m.NumChannels, nil
}

// Reset implements pipe.Resetter.
func (m *Pump) Reset(string) error {
	m.reset()
	return nil
}

// Process implementation for runner
func (m *Processor) Process(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) ([][]float64, error), error) {
	return func(b [][]float64) ([][]float64, error) {
		m.Advance(b)
		return b, nil
	}, nil
}

// Reset implements pipe.Resetter.
func (m *Processor) Reset(string) error {
	m.reset()
	return nil
}

// Sink implementation for runner.
func (m *Sink) Sink(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error) {
	return func(b [][]float64) error {
		m.buffer = signal.Float64(m.buffer).Append(b)
		m.Advance(b)
		return nil
	}, nil
}

// Reset implements pipe.Resetter.
func (m *Sink) Reset(string) error {
	m.buffer = nil
	m.reset()
	return nil
}

// Buffer returns sink's buffer
func (m *Sink) Buffer() signal.Float64 {
	return m.buffer
}

// Reset resets counter's metrics.
func (c *counter) reset() {
	c.messages, c.samples = 0, 0
}

// counter counts messages and samples.
// Duration is not zero only in context of measure.
type counter struct {
	messages int
	samples  int
}

// Advance counter's metrics.
func (c *counter) Advance(buf signal.Float64) {
	c.messages++
	c.samples = c.samples + buf.Size()
}

// Count returns messages and samples metrics.
func (c *counter) Count() (int, int) {
	return c.messages, c.samples
}

// Messages returns messages metrics.
func (c *counter) Messages() int {
	return c.messages
}
