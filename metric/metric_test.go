package metric_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono/metric"
)

func TestMeter(t *testing.T) {
	// test cases
	var tests = []struct {
		routines int
		counter  string
		inc      int64
		iter     int
		expected int64
	}{
		{
			routines: 2,
			inc:      10,
			iter:     100,
			expected: 10 * 100,
		},
		{
			routines: 10,
			inc:      5,
			iter:     100,
			expected: 5 * 100,
		},
		{
			routines: 100,
			inc:      5,
			iter:     100,
			expected: 5 * 100,
		},
	}

	// function to test meter.
	testFn := func(c *metric.Meter, wg *sync.WaitGroup, counter string, inc int64, iter int) {
		var v int64
		for i := 0; i < iter; i++ {
			v = v + inc
			c.Store(counter, v)
		}
		wg.Done()
	}

	m := metric.Metric{}
	for _, c := range tests {
		wg := &sync.WaitGroup{}
		wg.Add(c.routines)
		for i := 0; i < c.routines; i++ {
			meterName := fmt.Sprintf("test %d", i)
			meter := m.Meter(meterName, c.counter)
			go testFn(meter, wg, c.counter, c.inc, c.iter)
		}
		// check if no data race.
		_ = m.Measure()
		wg.Wait()
		metric := m.Measure()
		assert.Equal(t, c.routines, len(metric))
		for _, meters := range metric {
			assert.Equal(t, c.expected, meters[c.counter])
		}
	}
}
