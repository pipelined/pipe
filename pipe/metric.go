package pipe

import (
	"time"

	"github.com/pipelined/phono"
)

// Metrics types.
type (
	// Metric stores meters of pipe components.
	Metric interface {
		Meter(id string, counters ...string) Meter
		Measure() Measure
	}

	// Meter stores counters values.
	Meter interface {
		Store(counter string, value interface{})
		Load(counter string) interface{}
	}

	// Measure is a snapshot of full metric with all counters.
	Measure map[string]map[string]interface{}
)

type meter struct {
	Meter
	sampleRate  int
	startedAt   time.Time     // StartCounter
	messages    int64         // MessageCounter
	samples     int64         // SampleCounter
	latency     time.Duration // LatencyCounter
	processedAt time.Time
	elapsed     time.Duration // ElapsedCounter
	duration    time.Duration // DurationCounter
}

// newMeter creates new meter with counters.
func newMeter(componentID string, sampleRate int, m Metric) *meter {
	if m == nil {
		return nil
	}
	meter := meter{
		sampleRate:  sampleRate,
		startedAt:   time.Now(),
		processedAt: time.Now(),
	}
	if m != nil {
		meter.Meter = m.Meter(componentID, counters...)
		meter.Store(StartCounter, meter.startedAt)
	}

	return &meter
}

// message capture metrics after message is processed.
func (m *meter) message() *meter {
	if m == nil {
		return nil
	}
	m.messages++
	m.latency = time.Since(m.processedAt)
	m.processedAt = time.Now()
	m.elapsed = time.Since(m.startedAt)
	if m.Meter != nil {
		m.Store(MessageCounter, m.messages)
		m.Store(LatencyCounter, m.latency)
		m.Store(ElapsedCounter, m.elapsed)
	}
	return m
}

// sample capture metrics after samples are processed.
func (m *meter) sample(s int64) *meter {
	if m == nil {
		return nil
	}
	m.samples = m.samples + s
	m.duration = phono.DurationOf(m.sampleRate, m.samples)
	if m.Meter != nil {
		m.Store(SampleCounter, m.samples)
		m.Store(DurationCounter, m.duration)
	}
	return m
}

// counters is a structure for metrics initialization.
var counters = []string{MessageCounter, SampleCounter, StartCounter, LatencyCounter, DurationCounter, ElapsedCounter}
