package mock

import (
	"io"
	"time"

	"github.com/pipelined/phono"
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
	BufferSize  int
	NumChannels int
}

// Sink mocks up a pipe.Sink interface.
// Buffer is not thread-safe, so should not be checked while pipe is running.
type Sink struct {
	counter
	phono.Buffer
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
func (m *Pump) Pump(string) (func() (phono.Buffer, error), error) {
	return func() (phono.Buffer, error) {
		if Limit(m.Messages()) >= m.Limit {
			return nil, io.EOF
		}
		time.Sleep(m.Interval)

		b := phono.Buffer(make([][]float64, m.NumChannels))
		for i := range b {
			b[i] = make([]float64, m.BufferSize)
			for j := range b[i] {
				b[i][j] = m.Value
			}
		}
		m.counter.Advance(b)
		return b, nil
	}, nil
}

// Reset implements pipe.Resetter.
func (m *Pump) Reset(string) error {
	m.reset()
	return nil
}

// Process implementation for runner
func (m *Processor) Process(string) (func(phono.Buffer) (phono.Buffer, error), error) {
	return func(b phono.Buffer) (phono.Buffer, error) {
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
func (m *Sink) Sink(string) (func(phono.Buffer) error, error) {
	return func(b phono.Buffer) error {
		m.Buffer = m.Buffer.Append(b)
		m.Advance(b)
		return nil
	}, nil
}

// Reset implements pipe.Resetter.
func (m *Sink) Reset(string) error {
	m.Buffer = nil
	m.reset()
	return nil
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
func (c *counter) Advance(buf phono.Buffer) {
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
