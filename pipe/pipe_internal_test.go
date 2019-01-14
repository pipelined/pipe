package pipe

import (
	"testing"
	"time"

	"github.com/pipelined/phono"
	"github.com/pipelined/phono/mock"
	"github.com/stretchr/testify/assert"
)

func TestSimpleParams(t *testing.T) {
	pump := &mock.Pump{}
	interval := 10 * time.Millisecond
	p := params(make(map[string][]phono.ParamFunc))
	p = p.add(pump.IntervalParam(interval))
	p.applyTo(pump.ID())

	assert.Equal(t, interval, pump.Interval)
}

func TestMergeParams(t *testing.T) {
	var p, newP params
	pump := &mock.Pump{}

	interval := 10 * time.Millisecond
	p = make(map[string][]phono.ParamFunc)
	newP = make(map[string][]phono.ParamFunc)
	newP.add(pump.IntervalParam(interval))
	p = p.merge(newP)
	p.applyTo(pump.ID())
	assert.Equal(t, interval, pump.Interval)

	newInterval := 20 * time.Millisecond
	newP = make(map[string][]phono.ParamFunc)
	newP = newP.add(pump.IntervalParam(newInterval))
	p = p.merge(newP)
	p.applyTo(pump.ID())
	assert.Equal(t, newInterval, pump.Interval)
}
