package metric

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pipelined/phono/pipe"
)

// Metric contains component's Meters.
type Metric struct {
	m      sync.Mutex
	meters map[string]*Meter
}

// Meter read-only immutable map of atomic Counters.
type Meter struct {
	values map[string]*atomic.Value
}

// Meter returns new Meter for provided id. Existing Meter is flushed.
// If no match found, new Meter is added and returned.
func (m *Metric) Meter(id string, counters ...string) pipe.Meter {
	m.m.Lock()
	defer m.m.Unlock()

	// remove current meter.
	if m.meters == nil {
		m.meters = make(map[string]*Meter)
	} else {
		delete(m.meters, id)
	}

	// create new meter with provided counters
	meter := &Meter{values: make(map[string]*atomic.Value)}

	for _, counter := range counters {
		meter.values[counter] = &atomic.Value{}
	}

	m.meters[id] = meter
	return meter
}

// Measure returns Metric's measures.
func (m *Metric) Measure() pipe.Measure {
	r := make(map[string]map[string]interface{})
	m.m.Lock()
	defer m.m.Unlock()

	for meterName, meter := range m.meters {
		meterValues := make(map[string]interface{})
		for counterName, counter := range meter.values {
			meterValues[counterName] = counter.Load()
		}
		r[meterName] = meterValues
	}

	return r
}

// Store new counter value.
func (m *Meter) Store(c string, v interface{}) {
	if m == nil {
		return
	}

	if counter, ok := m.values[c]; ok {
		counter.Store(v)
	} else {
		panic(fmt.Sprintf("Counter %s does not exist", c))
	}
}

// Load counter value.
func (m *Meter) Load(c string) interface{} {
	if m == nil {
		return nil
	}

	if v, ok := m.values[c]; ok {
		return v.Load()
	}
	return nil
}
