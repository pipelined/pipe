package metric_test

import (
	"reflect"
	"sync"
	"testing"

	"pipelined.dev/pipe/metric"
	"pipelined.dev/signal"
)

func TestMeter(t *testing.T) {
	sampleRate := 44100
	pint := 1
	// test cases
	var tests = []struct {
		component          interface{}
		routines           int
		buffers            int
		bufferSize         int
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
	testFn := func(fn metric.ResetFunc, wg *sync.WaitGroup, buffers int, bufferSize int) {
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
		assertEqual(t, "samples", values[metric.SampleCounter], c.expectedSamples)
		assertEqual(t, "components", values[metric.ComponentCounter], c.expectedComponents)
	}

	total := metric.GetAll()
	assertEqual(t, "total", len(total), 2)
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
