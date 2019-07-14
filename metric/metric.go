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

const ComponentsLabel = "pipe.components"

var (
	components = metrics{
		m: make(map[string]metric),
	}
)

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
	key      string
	messages *expvar.Int
	samples  *expvar.Int
	latency  *duration
	duration *duration
}

func newMetric(componentType string) metric {
	m := metric{
		key:      componentType,
		messages: expvar.NewInt(Key(componentType, MessageCounter)),
		samples:  expvar.NewInt(Key(componentType, SampleCounter)),
		latency:  &duration{},
		duration: &duration{},
	}
	expvar.Publish(Key(componentType, LatencyCounter), m.latency)
	expvar.Publish(Key(componentType, DurationCounter), m.duration)
	return m
}

func Key(componentType, counter string) string {
	return fmt.Sprintf("%s.%s.%s", ComponentsLabel, componentType, counter)
}

// Meter creates new meter with component counters.
func Meter(component interface{}, sampleRate int) func(int64) {
	// get component type
	t := getType(component)
	// get metric
	metric := components.get(t)
	// reset time
	calledAt := time.Now()
	return func(s int64) {
		// increase message counter
		metric.messages.Add(1)
		// increase sample counter
		metric.samples.Add(s)
		// increase duration counter
		metric.duration.add(signal.DurationOf(sampleRate, s))
		// increase latency counter
		metric.latency.set(time.Since(calledAt))
		calledAt = time.Now()
	}
}

func getType(component interface{}) string {
	rv := reflect.ValueOf(component)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}
	return rv.Type().String()
}

const (
	// MessageCounter measures number of messages.
	MessageCounter = "Messages"
	// SampleCounter measures number of samples.
	SampleCounter = "Samples"
	// LatencyCounter measures latency between processing calls.
	LatencyCounter = "Latency"
	// DurationCounter counts what's the duration of signal.
	DurationCounter = "Duration"
)

// Duration allows to format time.Duration metric values.
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
