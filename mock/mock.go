package mock

import (
	"time"

	"github.com/dudk/phono"
)

// Param types
type (
	// Limit messages
	Limit int
)

// Pump mocks a pipe.Pump interface.
type Pump struct {
	phono.UID
	counter
	Interval time.Duration
	Limit
	Value float64
	phono.BufferSize
	phono.NumChannels
}

// Sink mocks up a pipe.Sink interface.
// Buffer is not thread-safe, so should not be checked while pipe is running.
type Sink struct {
	phono.UID
	counter
	phono.Buffer
}

// Processor mocks a pipe.Processor interface.
type Processor struct {
	phono.UID
	counter
}

// IntervalParam pushes new interval value for pump.
func (m *Pump) IntervalParam(i time.Duration) phono.Param {
	return phono.Param{
		ID: m.ID(),
		Apply: func() error {
			m.Interval = i
			return nil
		},
	}
}

// LimitParam pushes new limit value for pump.
func (m *Pump) LimitParam(l Limit) phono.Param {
	return phono.Param{
		ID: m.ID(),
		Apply: func() error {
			m.Limit = l
			return nil
		},
	}
}

// ValueParam pushes new signal value for pump.
func (m *Pump) ValueParam(v float64) phono.Param {
	return phono.Param{
		ID: m.ID(),
		Apply: func() error {
			m.Value = v
			return nil
		},
	}
}

// NumChannelsParam pushes new number of channels for pump
func (m *Pump) NumChannelsParam(nc phono.NumChannels) phono.Param {
	return phono.Param{
		ID: m.ID(),
		Apply: func() error {
			m.NumChannels = nc
			return nil
		},
	}
}

// Pump returns new buffer for pipe.
func (m *Pump) Pump(string) (phono.PumpFunc, error) {
	return func() (phono.Buffer, error) {
		if Limit(m.Messages()) >= m.Limit {
			return nil, phono.ErrEOP
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
func (m *Processor) Process(string) (phono.ProcessFunc, error) {
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
func (m *Sink) Sink(string) (phono.SinkFunc, error) {
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
	messages int64
	samples  int64
}

// Advance counter's metrics.
func (c *counter) Advance(buf phono.Buffer) {
	c.messages++
	c.samples = c.samples + int64(buf.Size())
}

// Reset resets counter's metrics.
func (c *counter) Reset() {
	c.messages, c.samples = 0, 0
}

// Count returns messages and samples metrics.
func (c *counter) Count() (int64, int64) {
	return c.messages, c.samples
}

// Messages returns messages metrics.
func (c *counter) Messages() int64 {
	return c.messages
}
