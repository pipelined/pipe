package metric_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/mock"
	"github.com/pipelined/phono/pipe"
	"github.com/pipelined/phono/pipe/metric"
)

func TestMeter(t *testing.T) {
	// test cases
	var tests = []struct {
		*metric.Metric
		routines int
		messages int
		samples  int64
		expected int64
	}{
		{
			Metric:   &metric.Metric{},
			routines: 2,
			messages: 10,
			samples:  100,
			expected: 10 * 100,
		},
		{
			Metric:   &metric.Metric{},
			routines: 10,
			messages: 5,
			samples:  100,
			expected: 5 * 100,
		},
		{
			routines: 100,
			messages: 5,
			samples:  100,
			expected: 0,
		},
	}

	// function to test meter.
	testFn := func(c *metric.Meter, wg *sync.WaitGroup, messages int, samples int64) {
		for i := 0; i < messages; i++ {
			c.Message().Sample(samples)
		}
		wg.Done()
	}

	for _, c := range tests {
		m := c.Metric
		wg := &sync.WaitGroup{}
		wg.Add(c.routines)
		for i := 0; i < c.routines; i++ {
			meterName := fmt.Sprintf("test %d", i)
			meter := m.Meter(meterName, 44100)
			go testFn(meter, wg, c.messages, c.samples)
		}
		// check if no data race.
		_ = m.Measure()
		wg.Wait()
		measure := m.Measure()
		for _, meters := range measure {
			assert.Equal(t, c.expected, meters[metric.SampleCounter])
		}
	}
}

func TestPipe(t *testing.T) {
	bufferSize := 10
	pump := &mock.Pump{
		SampleRate:  44100,
		Limit:       5,
		Interval:    10 * time.Microsecond,
		NumChannels: 1,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	sink1 := &mock.Sink{}
	sink2 := &mock.Sink{}
	m := &metric.Metric{}
	p, _ := pipe.New(
		bufferSize,
		pipe.WithMetric(m),
		pipe.WithName("Pipe"),
		pipe.WithPump(pump),
		pipe.WithProcessors(proc1, proc2),
		pipe.WithSinks(sink1, sink2),
	)
	pipe.Wait(p.Run())

	pumpID := p.ComponentID(pump)

	measure := m.Measure()
	assert.Equal(t, 1133786*time.Nanosecond, measure[pumpID][metric.DurationCounter])
	pipe.Wait(p.Run())
	measure = m.Measure()
	assert.Equal(t, 1133786*time.Nanosecond, measure[pumpID][metric.DurationCounter])
}
