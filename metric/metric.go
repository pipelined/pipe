package metric

import (
	"expvar"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pipelined/signal"
)

const componentsLabel = "pipe.components"

const (
	// MessageCounter measures number of messages.
	MessageCounter = "Messages"
	// SampleCounter measures number of samples.
	SampleCounter = "Samples"
	// LatencyCounter measures latency between processing calls.
	LatencyCounter = "Latency"
	// DurationCounter counts what's the duration of signal.
	DurationCounter = "Duration"
	// ComponentCounter counts number of calls.
	ComponentCounter = "Components"
)

var (
	components = metrics{
		m: make(map[string]metric),
	}

	counters = []string{
		MessageCounter,
		SampleCounter,
		LatencyCounter,
		DurationCounter,
		ComponentCounter,
	}
)

// Get metrics values for provided component type.
func Get(component interface{}) map[string]string {
	return getCounters(getType(component))
}

// GetAll returns counters for all measured components.
func GetAll() map[string]map[string]string {
	m := make(map[string]map[string]string)
	components.Lock()
	defer components.Unlock()
	for component := range components.m {
		m[component] = getCounters(component)
	}
	return m
}

func getCounters(componentType string) map[string]string {
	m := make(map[string]string)
	for _, counter := range counters {
		v := expvar.Get(key(componentType, counter))
		if v != nil {
			m[counter] = v.String()
		}
	}
	return m
}

// ResetFunc returns new Measure closure. This closure is needed to postpone metrics
// capture until component is actually running.
type ResetFunc func() MeasureFunc

// MeasureFunc captures metrics when buffer is processed.
type MeasureFunc func(bufferSize int64)

// Meter creates new meter closure to capture component counters.
func Meter(component interface{}, sampleRate int) ResetFunc {
	t := getType(component)
	metric := components.get(t)
	metric.components.Add(1)
	return func() MeasureFunc {
		calledAt := time.Now()
		var (
			bufferSize     int64
			bufferDuration time.Duration
		)
		return func(s int64) {
			metric.latency.set(time.Since(calledAt))
			metric.messages.Add(1)
			metric.samples.Add(s)
			// recalculate buffer duration only when buffer size has changed
			if bufferSize != s {
				bufferSize = s
				bufferDuration = signal.DurationOf(sampleRate, s)
			}
			metric.duration.add(bufferDuration)
			calledAt = time.Now()
		}
	}
}

type metrics struct {
	sync.Mutex
	m map[string]metric
}

func (m *metrics) get(componentType string) metric {
	m.Lock()
	defer m.Unlock()
	if metric, ok := m.m[componentType]; ok {
		// return existing metric if available
		return metric
	}
	// create new metric
	metric := newMetric(componentType)
	m.m[componentType] = metric
	return metric
}

type metric struct {
	key        string
	components *expvar.Int
	messages   *expvar.Int
	samples    *expvar.Int
	latency    *duration
	duration   *duration
}

func newMetric(componentType string) metric {
	m := metric{
		key:        componentType,
		components: expvar.NewInt(key(componentType, ComponentCounter)),
		messages:   expvar.NewInt(key(componentType, MessageCounter)),
		samples:    expvar.NewInt(key(componentType, SampleCounter)),
		latency:    &duration{},
		duration:   &duration{},
	}
	expvar.Publish(key(componentType, LatencyCounter), m.latency)
	expvar.Publish(key(componentType, DurationCounter), m.duration)
	return m
}

func key(componentType, counter string) string {
	return fmt.Sprintf("%s.%s.%s", componentsLabel, componentType, counter)
}

func getType(component interface{}) string {
	rv := reflect.ValueOf(component)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}
	return rv.Type().String()
}

// duration allows to format time.Duration metric values.
type duration struct {
	d int64
}

func (v *duration) String() string {
	return fmt.Sprintf("%v", time.Duration(atomic.LoadInt64(&v.d)))
}

func (v *duration) add(delta time.Duration) {
	atomic.AddInt64(&v.d, int64(delta))
}

func (v *duration) set(value time.Duration) {
	atomic.StoreInt64(&v.d, int64(value))
}
