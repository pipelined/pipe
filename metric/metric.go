package metric

import (
	"sync/atomic"
	"time"
)

// Counter is an atomic int64 counter.
type Counter struct {
	label string
	value int64
}

// Inc increases value by defined delta.
func (c *Counter) Inc(v int64) {
	atomic.AddInt64(&c.value, v)
}

// Value returns current counter value.
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// DurationCounter is an atomic time.Duration counter.
type DurationCounter struct {
	counter Counter
}

// Inc increases value by defined delta.
func (c *DurationCounter) Inc(v time.Duration) {
	c.counter.Inc(int64(v))
}

// Value returns current counter value.
func (c *DurationCounter) Value() time.Duration {
	return time.Duration(c.counter.Value())
}
