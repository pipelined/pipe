package metric_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/phono/metric"
	"github.com/pipelined/phono/mock"
	"github.com/pipelined/phono/pipe"
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
	testFn := func(c pipe.Meter, wg *sync.WaitGroup, counter string, inc int64, iter int) {
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

func TestInvalidCounter(t *testing.T) {
	m := metric.Metric{}
	meterName, counterName, invalidCounter := "testMeter", "testCounter", "invalidCounter"
	meter := m.Meter(meterName, counterName)

	assert.Panics(t, func() { meter.Store(invalidCounter, 1) })

	v := meter.Load(invalidCounter)
	assert.Nil(t, v)
}

func TestPipe(t *testing.T) {
	sampleRate := 44100
	pump := &mock.Pump{
		Limit:       5,
		Interval:    10 * time.Microsecond,
		BufferSize:  10,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}
	metric := &metric.Metric{}
	p, _ := pipe.New(
		sampleRate,
		pipe.WithMetric(metric),
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)
	pipe.Wait(p.Run())

	pumpID := p.ComponentID(pump)

	m := metric.Measure()
	assert.Equal(t, 1133786*time.Nanosecond, m[pumpID][pipe.DurationCounter])
	pipe.Wait(p.Run())
	m = metric.Measure()
	assert.Equal(t, 1133786*time.Nanosecond, m[pumpID][pipe.DurationCounter])
}
