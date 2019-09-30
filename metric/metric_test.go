package metric_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe/metric"
	"github.com/pipelined/signal"
)

func TestMeter(t *testing.T) {
	sampleRate := 44100
	pint := 1
	// test cases
	var tests = []struct {
		component          interface{}
		routines           int
		buffers            int
		bufferSize         int64
		expectedSamples    string
		expectedComponents string
	}{
		{
			component:          int(1),
			routines:           2,
			buffers:            10,
			bufferSize:         100,
			expectedSamples:    "2000",
			expectedComponents: "2",
		},
		{
			component:          &pint,
			routines:           2,
			buffers:            10,
			bufferSize:         100,
			expectedSamples:    "4000",
			expectedComponents: "4",
		},
		{
			component:          "test",
			routines:           2,
			buffers:            10,
			bufferSize:         100,
			expectedSamples:    "2000",
			expectedComponents: "2",
		},
	}
	// function to test meter.
	testFn := func(fn metric.ResetFunc, wg *sync.WaitGroup, buffers int, bufferSize int64) {
		m := fn()
		for i := 0; i < buffers; i++ {
			m(bufferSize)
		}
		wg.Done()
	}

	for _, c := range tests {
		wg := &sync.WaitGroup{}
		wg.Add(c.routines)
		for i := 0; i < c.routines; i++ {
			go testFn(metric.Meter(c.component, signal.SampleRate(sampleRate)), wg, c.buffers, c.bufferSize)
		}
		// check if no data race.
		wg.Wait()
		values := metric.Get(c.component)
		assert.Equal(t, c.expectedSamples, values[metric.SampleCounter])
		assert.Equal(t, c.expectedComponents, values[metric.ComponentCounter])
	}

	total := metric.GetAll()
	assert.Equal(t, 2, len(total))
}
