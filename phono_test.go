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
	params = params.Merge(newParams)
	params.ApplyTo(p)
	assert.Equal(t, interval, p.Interval)

	newInterval := mock.Interval(20)
	newParams = phono.NewParams(p.IntervalParam(newInterval))
	params = params.Merge(newParams)
	params.ApplyTo(p)
	assert.Equal(t, newInterval, p.Interval)
}

func TestBuffer(t *testing.T) {
	var s phono.Buffer
	assert.Equal(t, phono.NumChannels(0), s.NumChannels())
	assert.Equal(t, phono.BufferSize(0), s.Size())
	s = phono.NewBuffer(1, 0)
	assert.Equal(t, phono.NumChannels(1), s.NumChannels())
	assert.Equal(t, phono.BufferSize(0), s.Size())
	s[0] = make([]float64, 512)
	assert.Equal(t, phono.BufferSize(512), s.Size())

	s2 := phono.NewBuffer(phono.NumChannels(1), phono.BufferSize(512))
	s = s.Append(s2)
	assert.Equal(t, phono.BufferSize(512), s.Size())
	s2[0] = make([]float64, 512)
	s = s.Append(s2)
	assert.Equal(t, phono.BufferSize(1024), s.Size())
}
