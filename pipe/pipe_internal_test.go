package pipe

import (
	"testing"
	"time"

	"github.com/pipelined/phono/mock"
	"github.com/stretchr/testify/assert"
)

func TestSimpleParams(t *testing.T) {
	pump := &mock.Pump{}
	interval := 10 * time.Millisecond
	p := params(make(map[string][]func()))
	p = p.add(pump.ID(), pump.IntervalParam(interval))
	p.applyTo(pump.ID())

	assert.Equal(t, interval, pump.Interval)
}

func TestMergeParams(t *testing.T) {
	var p, newP params
	pump := &mock.Pump{}

	interval := 10 * time.Millisecond
	p = make(map[string][]func())
	newP = make(map[string][]func())
	newP.add(pump.ID(), pump.IntervalParam(interval))
	p = p.merge(newP)
	p.applyTo(pump.ID())
	assert.Equal(t, interval, pump.Interval)

	newInterval := 20 * time.Millisecond
	newP = make(map[string][]func())
	newP = newP.add(pump.ID(), pump.IntervalParam(newInterval))
	p = p.merge(newP)
	p.applyTo(pump.ID())
	assert.Equal(t, newInterval, pump.Interval)
}
