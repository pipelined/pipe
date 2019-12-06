// Package mock provides mocks for pipeline components and allows to execute integration tests.
package mock

import (
	"io"
	"time"

	"pipelined.dev/signal"
)

// Pump mocks a pipe.Pump interface.
type Pump struct {
	counter
	Interval    time.Duration
	Limit       int
	Value       float64
	NumChannels int
	SampleRate  signal.SampleRate
	ErrorOnCall error
	Hooks
}

// IntervalParam pushes new interval value for pump.
func (m *Pump) IntervalParam(i time.Duration) func() {
	return func() {
		m.Interval = i
	}
}

// LimitParam pushes new limit value for pump.
func (m *Pump) LimitParam(l int) func() {
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

// Pump returns new buffer for pipe.
func (m *Pump) Pump(sourceID string) (func(b signal.Float64) error, signal.SampleRate, int, error) {
	return func(b signal.Float64) error {
		if m.ErrorOnCall != nil {
			return m.ErrorOnCall
		}

		if m.samples >= m.Limit {
			return io.EOF
		}
		time.Sleep(m.Interval)

		// calculate buffer size.
		bs := b.Size()
		// check if we need a shorter.
		if left := m.Limit - m.samples; left < bs {
			bs = left
		}
		for i := range b {
			// resize buffer
			b[i] = b[i][:bs]
			for j := range b[i] {
				b[i][j] = m.Value
			}
		}
		m.advance(bs)
		return nil
	}, m.SampleRate, m.NumChannels, nil
}

// Reset implements pipe.Resetter.
func (m *Pump) Reset(string) error {
	m.Resetted = true
	if m.ErrorOnReset != nil {
		return m.ErrorOnReset
	}
	m.reset()
	return nil
}

// Interrupt implements pipe.Interrupter.
func (m *Pump) Interrupt(string) error {
	m.Interrupted = true
	return m.ErrorOnInterrupt
}

// Flush implements pipe.Flusher.
func (m *Pump) Flush(string) error {
	m.Flushed = true
	return m.ErrorOnFlush
}

// Processor mocks a pipe.Processor interface.
type Processor struct {
	counter
	ErrorOnCall error
	Hooks
}

// Process implementation for runner
func (m *Processor) Process(pipeID string, sampleRate signal.SampleRate, numChannels int) (func(signal.Float64) error, error) {
	return func(b signal.Float64) error {
		if m.ErrorOnCall != nil {
			return m.ErrorOnCall
		}
		m.advance(b.Size())
		return nil
	}, nil
}

// Reset implements pipe.Resetter.
func (m *Processor) Reset(string) error {
	m.Resetted = true
	if m.ErrorOnReset != nil {
		return m.ErrorOnReset
	}
	m.reset()
	return nil
}

// Interrupt implements pipe.Interrupter.
func (m *Processor) Interrupt(string) error {
	m.Interrupted = true
	return m.ErrorOnInterrupt
}

// Flush implements pipe.Flusher.
func (m *Processor) Flush(string) error {
	m.Flushed = true
	return m.ErrorOnFlush
}

// Sink mocks up a pipe.Sink interface.
// Buffer is not thread-safe, so should not be checked while pipe is running.
type Sink struct {
	counter
	buffer      signal.Float64
	Discard     bool
	ErrorOnCall error
	Hooks
}

// Sink implementation for runner.
func (m *Sink) Sink(pipeID string, sampleRate signal.SampleRate, numChannels int) (func(signal.Float64) error, error) {
	return func(b signal.Float64) error {
		if m.ErrorOnCall != nil {
			return m.ErrorOnCall
		}
		if !m.Discard {
			m.buffer = signal.Float64(m.buffer).Append(b)
		}
		m.advance(b.Size())
		return nil
	}, nil
}

// Reset implements pipe.Resetter.
func (m *Sink) Reset(string) error {
	m.Resetted = true
	m.buffer = nil
	m.reset()
	return m.ErrorOnReset
}

// Interrupt implements pipe.Interrupter.
func (m *Sink) Interrupt(string) error {
	m.Interrupted = true
	return m.ErrorOnInterrupt
}

// Flush implements pipe.Flusher.
func (m *Sink) Flush(string) error {
	m.Flushed = true
	return m.ErrorOnFlush
}

// Hooks allows to mock components hooks.
type Hooks struct {
	Resetted    bool
	Flushed     bool
	Interrupted bool

	ErrorOnReset     error
	ErrorOnFlush     error
	ErrorOnInterrupt error
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
func (c *counter) advance(size int) {
	c.messages++
	c.samples = c.samples + size
}

// Count returns messages and samples metrics.
func (c *counter) Count() (int, int) {
	return c.messages, c.samples
}
