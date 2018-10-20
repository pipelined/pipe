package pipe

import (
	"fmt"
	"time"

	"github.com/dudk/phono"
)

type (
	// measurable identifies an entity with measurable metrics.
	// Each measurable can have multiple counters.
	// If custom metrics will be needed, it can be exposed in the future.
	measurable interface {
		Reset()
		Measure() Measure
		FinishMeasure()
		Counter(string) *Counter
		Latency()
	}

	// metric represents measures of pipe components.
	metric struct {
		ID string
		phono.SampleRate
		Counters       map[string]*Counter
		start          time.Time
		elapsed        time.Duration
		latencyMeasure time.Time
		latency        time.Duration
	}

	// Measure represents metric values at certain moment of time.
	// It should be used to transfer metrics values through pipe and avoid data races with counters.
	Measure struct {
		ID string
		phono.SampleRate
		Counters map[string]Counter
		Start    time.Time
		Elapsed  time.Duration
		Latency  time.Duration
	}

	// Counter counts messages and samples.
	// Duration is not zero only in context of measure.
	Counter struct {
		messages int64
		samples  int64
		duration time.Duration
	}
)

// newMetric creates new metric with requested measures.
func newMetric(id string, sampleRate phono.SampleRate, keys ...string) *metric {
	m := &metric{
		ID:         id,
		SampleRate: sampleRate,
		Counters:   make(map[string]*Counter),
	}
	m.AddCounters(keys...)
	return m
}

// Reset metrics values
func (m *metric) Reset() {
	m.start = time.Now()
	m.latencyMeasure = time.Now()
	for key := range m.Counters {
		m.Counters[key].Reset()
	}
}

// FinishMeasure finalizes metric values.
func (m *metric) FinishMeasure() {
	m.elapsed = time.Since(m.start)
}

// Counter returns counter for specified key.
func (m *metric) Counter(key string) *Counter {
	return m.Counters[key]
}

// AddCounters to metric with defined keys.
func (m *metric) AddCounters(keys ...string) {
	for _, measure := range keys {
		m.Counters[measure] = &Counter{}
	}
}

// Latency sets latency since last latency measure.
func (m *metric) Latency() {
	m.latency = time.Since(m.latencyMeasure)
	m.latencyMeasure = time.Now()
}

// String returns string representation of Metrics.
func (m Measure) String() string {
	return fmt.Sprintf("SampleRate: %v Started: %v Elapsed: %v Latency: %v Counters: %v", m.SampleRate, m.Start, m.Elapsed, m.Latency, m.Counters)
}

// Measure returns latest measures of metric.
func (m *metric) Measure() Measure {
	if m == nil {
		return Measure{}
	}
	elapsed := m.elapsed
	if !m.start.IsZero() && elapsed == 0 {
		elapsed = time.Since(m.start)
	}
	measure := Measure{
		ID:         m.ID,
		SampleRate: m.SampleRate,
		Start:      m.start,
		Elapsed:    elapsed,
		Latency:    m.latency,
		Counters:   make(map[string]Counter),
	}
	for key, counter := range m.Counters {
		c := *counter
		c.duration = m.SampleRate.DurationOf(counter.samples)
		measure.Counters[key] = c
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

// Duration returns duration of counted samples.
func (c *Counter) Duration() time.Duration {
	return c.duration
}

// String representation of Counter.
func (c Counter) String() string {
	return fmt.Sprintf("Messages: %v Samples: %v Duration: %v", c.messages, c.samples, c.duration)
}
