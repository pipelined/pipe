package pipe

import (
	"fmt"
	"time"

	"github.com/dudk/phono"
)

type (
	// Measurable identifies an entity with measurable metrics.
	// Each measurable can have multiple counters.
	Measurable interface {
		Measure() Measure
		FinishMeasure()
		Counter(string) *Counter
	}

	// Metric represents measures of pipe components.
	Metric struct {
		phono.SampleRate
		Counters map[string]*Counter
		start    time.Time
		elapsed  time.Duration
		latency  time.Duration
	}

	// Measure represents Metric values at certain moment of time.
	// It should be used to transfer metrics values through pipe and avoid data races with counters.
	Measure struct {
		phono.SampleRate
		Counters map[string]Counter
		start    time.Time
		elapsed  time.Duration
		latency  time.Duration
	}

	// Counter counts messages and samples.
	Counter struct {
		messages int64
		samples  int64
	}
)

// NewMetric creates new metric with requested measures.
func NewMetric(sampleRate phono.SampleRate, measures ...string) *Metric {
	m := &Metric{
		SampleRate: sampleRate,
		Counters:   make(map[string]*Counter),
		start:      time.Now(),
	}
	for _, measure := range measures {
		m.Counters[measure] = &Counter{}
	}
	return m
}

// FinishMeasure finalizes metric values.
func (m *Metric) FinishMeasure() {
	m.elapsed = time.Since(m.start)
}

// Counter returns counter for specified key.
func (m *Metric) Counter(key string) *Counter {
	return m.Counters[key]
}

// String returns string representation of Metrics.
func (m Measure) String() string {
	return fmt.Sprintf("SampleRate: %v Started: %v Elapsed: %v", m.SampleRate, m.start, m.elapsed)
}

// Measure returns latest measures of Metric.
func (m *Metric) Measure() Measure {
	if m == nil {
		return Measure{}
	}
	elapsed := m.elapsed
	if elapsed == 0 {
		elapsed = time.Since(m.start)
	}
	measure := Measure{
		SampleRate: m.SampleRate,
		start:      m.start,
		elapsed:    elapsed,
		latency:    m.latency,
		Counters:   make(map[string]Counter),
	}
	for key, counter := range m.Counters {
		measure.Counters[key] = *counter
	}
	return measure
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
