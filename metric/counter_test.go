package metric_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono/metric"
)

var counterTests = []struct {
	routines int
	inc      int64
	iter     int
	expected int64
}{
	{
		routines: 2,
		inc:      10,
		iter:     100,
		expected: 2 * 10 * 100,
	},
	{
		routines: 10,
		inc:      5,
		iter:     100,
		expected: 10 * 5 * 100,
	},
	{
		routines: 100,
		inc:      5,
		iter:     100,
		expected: 100 * 5 * 100,
	},
}

var durationCounterTests = []struct {
	routines int
	inc      time.Duration
	iter     int
	expected time.Duration
}{
	{
		routines: 2,
		inc:      time.Second * 10,
		iter:     100,
		expected: time.Second * 2 * 10 * 100,
	},
	{
		routines: 10,
		inc:      time.Minute * 5,
		iter:     100,
		expected: time.Minute * 10 * 5 * 100,
	},
	{
		routines: 100,
		inc:      time.Millisecond * 5,
		iter:     100,
		expected: time.Millisecond * 100 * 5 * 100,
	},
}

func TestCounter(t *testing.T) {
	// function to test counter
	fn := func(c *metric.Counter, wg *sync.WaitGroup, inc int64, iter int) {
		for i := 0; i < iter; i++ {
			c.Inc(inc)
		}
		wg.Done()
	}
	for _, c := range counterTests {
		counter := &metric.Counter{}
		wg := &sync.WaitGroup{}
		wg.Add(c.routines)
		for i := 0; i < c.routines; i++ {
			go fn(counter, wg, c.inc, c.iter)
		}
		wg.Wait()
		assert.Equal(t, c.expected, counter.Value())
	}
}

func TestDurationCounter(t *testing.T) {
	// function to test counter
	fn := func(c *metric.DurationCounter, wg *sync.WaitGroup, inc time.Duration, iter int) {
		for i := 0; i < iter; i++ {
			c.Inc(inc)
		}
		wg.Done()
	}
	for _, c := range durationCounterTests {
		counter := &metric.DurationCounter{}
		wg := &sync.WaitGroup{}
		wg.Add(c.routines)
		for i := 0; i < c.routines; i++ {
			go fn(counter, wg, c.inc, c.iter)
		}
		wg.Wait()
		assert.Equal(t, c.expected, counter.Value())
	}
}
