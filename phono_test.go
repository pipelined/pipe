package phono_test

import (
	"testing"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/stretchr/testify/assert"
)

func TestSimpleParams(t *testing.T) {
	p := &mock.Pump{}
	interval := mock.Interval(10)
	params := phono.NewParams(p.IntervalParam(interval))
	params.ApplyTo(p)

	assert.Equal(t, interval, p.Interval)
}

func TestMergeParams(t *testing.T) {
	var params *phono.Params
	p := &mock.Pump{}

	interval := mock.Interval(10)
	newParams := phono.NewParams(p.IntervalParam(interval))
	params = params.Join(newParams)
	params.ApplyTo(p)
	assert.Equal(t, interval, p.Interval)

	newInterval := mock.Interval(20)
	newParams = phono.NewParams(p.IntervalParam(newInterval))
	params = params.Join(newParams)
	params.ApplyTo(p)
	assert.Equal(t, newInterval, p.Interval)
}
