package pipe

import (
	"testing"
	"time"

	"github.com/dudk/phono/mock"
	"github.com/stretchr/testify/assert"
)

func TestSimpleParams(t *testing.T) {
	pump := &mock.Pump{}
	interval := 10 * time.Millisecond
	p := newParams(pump.IntervalParam(interval))
	p.applyTo(pump.ID())

	assert.Equal(t, interval, pump.Interval)
}

func TestMergeParams(t *testing.T) {
	var p *params
	pump := &mock.Pump{}

	interval := 10 * time.Millisecond
	newP := newParams(pump.IntervalParam(interval))
	p = p.merge(newP)
	p.applyTo(pump.ID())
	assert.Equal(t, interval, pump.Interval)

	newInterval := 20 * time.Millisecond
	newP = newParams(pump.IntervalParam(newInterval))
	p = p.merge(newP)
	p.applyTo(pump.ID())
	assert.Equal(t, newInterval, pump.Interval)
}
