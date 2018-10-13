package pipe

import (
	"fmt"
	"time"

	"github.com/dudk/phono"
)

type (
	// Metric represents stats of runner
	Metric struct {
		phono.SampleRate
		Measures map[string]*Counter
		start    time.Time
		elapsed  time.Duration
		latency  time.Duration
	}

	// Counter can be used to measure passthrough
	Counter struct {
		messages int64
		samples  int64
	}
)

// NewMetric creates new metric with requested measures
func NewMetric(sampleRate phono.SampleRate, measures ...string) *Metric {
	m := &Metric{
		SampleRate: sampleRate,
		Measures:   make(map[string]*Counter),
		start:      time.Now(),
	}
	for _, measure := range measures {
		m.Measures[measure] = &Counter{}
	}
	return m
}

// Stop the metrics measures
func (m *Metric) Stop() {
	m.elapsed = time.Since(m.start)
}

// String returns string representation of Metrics
func (m *Metric) String() string {
	return fmt.Sprintf("SampleRate: %v Started: %v Elapsed:%v", m.SampleRate, m.start, m.elapsed)
}

// Advance counter's metrics.
func (c *Counter) Advance(buf phono.Buffer) {
	c.messages++
	c.samples = c.samples + int64(buf.Size())
}

// Reset resets counter's metrics.
func (c *Counter) Reset() {
	c.messages, c.samples = 0, 0
}

// Count returns messages and samples metrics.
func (c *Counter) Count() (int64, int64) {
	return c.messages, c.samples
}

// Messages returns messages metrics.
func (c *Counter) Messages() int64 {
	return c.messages
}

// Samples returns samples metrics.
func (c *Counter) Samples() int64 {
	return c.samples
}
