package metric

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pipelined/phono/signal"
)

// Metric contains component's Meters.
type Metric struct {
	m      sync.Mutex
	meters map[string]map[string]*atomic.Value
}

// counters read-only immutable map of atomic Counters.
// type counters map[string]*atomic.Value

// Measure is a snapshot of full metric with all counters.
type Measure map[string]map[string]interface{}

// addCounters to the metric. Metric used to generate measures for all counters.
//
// If id matches with existing counters, those will be replaced with the new one.
// If no match found, new counters is added and returned.
func (m *Metric) addCounters(id string, counters ...string) map[string]*atomic.Value {
	m.m.Lock()
	defer m.m.Unlock()

	// remove current meter.
	if m.meters == nil {
		m.meters = make(map[string]map[string]*atomic.Value)
	} else {
		delete(m.meters, id)
	}

	// create new meter with provided counters
	meter := make(map[string]*atomic.Value)

	for _, counter := range counters {
		meter[counter] = &atomic.Value{}
	}

	m.meters[id] = meter
	return meter
}

// Measure returns Metric's measures.
func (m *Metric) Measure() Measure {
	if m == nil {
		return nil
	}
	r := make(map[string]map[string]interface{})
	m.m.Lock()
	defer m.m.Unlock()

	for meterName, meter := range m.meters {
		meterValues := make(map[string]interface{})
		for counterName, counter := range meter {
			meterValues[counterName] = counter.Load()
		}
		r[meterName] = meterValues
	}

	return r
}

// Meter creates new meter with component counters.
func (m *Metric) Meter(componentID string, sampleRate int) *Meter {
	if m == nil {
		return nil
	}
	meter := Meter{
		sampleRate:  sampleRate,
		startedAt:   time.Now(),
		processedAt: time.Now(),
	}

	meter.counters = m.addCounters(componentID, componentCounters...)
	store(meter.counters, StartCounter, meter.startedAt)

	return &meter
}

// Meter contains all component's counters.
type Meter struct {
	counters    map[string]*atomic.Value
	sampleRate  int
	startedAt   time.Time     // StartCounter
	messages    int64         // MessageCounter
	samples     int64         // SampleCounter
	latency     time.Duration // LatencyCounter
	processedAt time.Time
	elapsed     time.Duration // ElapsedCounter
	duration    time.Duration // DurationCounter
}

// Message capture metrics after message is processed.
func (m *Meter) Message() *Meter {
	if m == nil {
		return nil
	}
	m.messages++
	m.latency = time.Since(m.processedAt)
	m.processedAt = time.Now()
	m.elapsed = time.Since(m.startedAt)

	store(m.counters, MessageCounter, m.messages)
	store(m.counters, LatencyCounter, m.latency)
	store(m.counters, ElapsedCounter, m.elapsed)

	return m
}

// Sample capture metrics after samples are processed.
func (m *Meter) Sample(s int64) *Meter {
	if m == nil {
		return nil
	}
	m.samples = m.samples + s
	m.duration = signal.DurationOf(m.sampleRate, m.samples)

	store(m.counters, SampleCounter, m.samples)
	store(m.counters, DurationCounter, m.duration)

	return m
}

const (
	// MessageCounter measures number of messages.
	MessageCounter = "Messages"
	// SampleCounter measures number of samples.
	SampleCounter = "Samples"
	// StartCounter fixes when runner started.
	StartCounter = "Start"
	// LatencyCounter measures latency between processing calls.
	LatencyCounter = "Latency"
	// ElapsedCounter fixes when runner ended.
	ElapsedCounter = "Elapsed"
	// DurationCounter counts what's the duration of signal.
	DurationCounter = "Duration"
)

// counters is a structure for metrics initialization.
var componentCounters = []string{MessageCounter, SampleCounter, StartCounter, LatencyCounter, DurationCounter, ElapsedCounter}

// Store new counter value.
func store(m map[string]*atomic.Value, c string, v interface{}) {
	if counter, ok := m[c]; ok {
		counter.Store(v)
	}
}
