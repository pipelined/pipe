package metric_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe/metric"
)

func TestMeter(t *testing.T) {
	// test cases
	var tests = []struct {
		meter      func(int64)
		routines   int
		buffers    int
		bufferSize int64
		expected   int64
	}{
		{
			meter:      metric.Meter(1, 44100),
			routines:   2,
			buffers:    10,
			bufferSize: 100,
			expected:   10 * 100,
		},
		// {
		// 	Metric:   &metric.Metric{},
		// 	routines: 10,
		// 	messages: 5,
		// 	samples:  100,
		// 	expected: 5 * 100,
		// },
		// {
		// 	Metric:   &metric.Metric{},
		// 	routines: 100,
		// 	messages: 5,
		// 	samples:  100,
		// 	expected: 0,
		// },
	}

	// m := metric.Metric{}

	// function to test meter.
	testFn := func(fn func(int64), wg *sync.WaitGroup, buffers int, bufferSize int64) {
		for i := 0; i < buffers; i++ {
			fn(bufferSize)
		}
		wg.Done()
	}

	for _, c := range tests {
		wg := &sync.WaitGroup{}
		wg.Add(c.routines)
		for i := 0; i < c.routines; i++ {
			meterName := fmt.Sprintf("test %d", i)
			go testFn(c.meter, wg, c.buffers, c.bufferSize)
		}
		// check if no data race.
		// _ = m.Measure()
		wg.Wait()
		// measure := m.Measure()
		for _, meters := range measure {
			assert.Equal(t, c.expected, meters[metric.SampleCounter])
		}
	}
}

// func TestPipe(t *testing.T) {
// 	bufferSize := 10
// 	limit := 55

// 	pump := &mock.Pump{
// 		SampleRate:  44100,
// 		Limit:       limit,
// 		Interval:    10 * time.Microsecond,
// 		NumChannels: 1,
// 	}
// 	proc1 := &mock.Processor{}
// 	proc2 := &mock.Processor{}
// 	sink1 := &mock.Sink{}
// 	sink2 := &mock.Sink{}
// 	m := &metric.Metric{}
// 	n, err := pipe.Network(
// 		&pipe.Pipe{
// 			Pump:       pump,
// 			Processors: pipe.Processors(proc1, proc2),
// 			Sinks:      pipe.Sinks(sink1, sink2),
// 		},
// 	)
// 	assert.Nil(t, err)
// 	pipe.Wait(n.Run(bufferSize))

// 	pumpID, ok := n.ComponentID(pump)
// 	assert.True(t, ok)
// 	assert.NotEqual(t, "", pumpID)

// 	measure := m.Measure()
// 	assert.Equal(t, 1247165*time.Nanosecond, measure[pumpID][metric.DurationCounter])
// 	pipe.Wait(n.Run(bufferSize))
// 	measure = m.Measure()
// 	assert.Equal(t, 1247165*time.Nanosecond, measure[pumpID][metric.DurationCounter])
// 	assert.Equal(t, int64(limit), measure[pumpID][metric.SampleCounter])
// 	assert.Equal(t, int64(math.Ceil(float64(limit)/float64(bufferSize))), measure[pumpID][metric.MessageCounter])
// }
