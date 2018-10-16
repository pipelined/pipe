package phono_test

import (
	"testing"
	"time"

	"github.com/dudk/phono"
	"github.com/dudk/phono/mock"
	"github.com/stretchr/testify/assert"
)

var sliceTests = []struct {
	in       phono.Buffer
	start    int64
	len      int
	expected phono.Buffer
}{
	{
		in:       phono.Buffer([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
		start:    1,
		len:      2,
		expected: phono.Buffer([][]float64{[]float64{1, 2}, []float64{1, 2}}),
	},
	{
		in:       phono.Buffer([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
		start:    5,
		len:      2,
		expected: phono.Buffer([][]float64{[]float64{5, 6}, []float64{5, 6}}),
	},
	{
		in:       phono.Buffer([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
		start:    7,
		len:      4,
		expected: phono.Buffer([][]float64{[]float64{7, 8, 9}, []float64{7, 8, 9}}),
	},
	{
		in:       phono.Buffer([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
		start:    9,
		len:      1,
		expected: phono.Buffer([][]float64{[]float64{9}}),
	},
	{
		in:       phono.Buffer([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
		start:    10,
		len:      1,
		expected: nil,
	},
}

func TestSimpleParams(t *testing.T) {
	p := &mock.Pump{}
	interval := 10 * time.Millisecond
	params := phono.NewParams(p.IntervalParam(interval))
	params.ApplyTo(p.ID())

	assert.Equal(t, interval, p.Interval)
}

func TestMergeParams(t *testing.T) {
	var params *phono.Params
	p := &mock.Pump{}

	interval := 10 * time.Millisecond
	newParams := phono.NewParams(p.IntervalParam(interval))
	params = params.Merge(newParams)
	params.ApplyTo(p.ID())
	assert.Equal(t, interval, p.Interval)

	newInterval := 20 * time.Millisecond
	newParams = phono.NewParams(p.IntervalParam(newInterval))
	params = params.Merge(newParams)
	params.ApplyTo(p.ID())
	assert.Equal(t, newInterval, p.Interval)
}

func TestBuffer(t *testing.T) {
	var s phono.Buffer
	assert.Equal(t, phono.NumChannels(0), s.NumChannels())
	assert.Equal(t, phono.BufferSize(0), s.Size())
	s = [][]float64{[]float64{}}
	assert.Equal(t, phono.NumChannels(1), s.NumChannels())
	assert.Equal(t, phono.BufferSize(0), s.Size())
	s[0] = make([]float64, 512)
	assert.Equal(t, phono.BufferSize(512), s.Size())

	s2 := [][]float64{make([]float64, 512)}
	s = s.Append(s2)
	assert.Equal(t, phono.BufferSize(1024), s.Size())
	s2[0] = make([]float64, 1024)
	s = s.Append(s2)
	assert.Equal(t, phono.BufferSize(2048), s.Size())
}

func TestSliceBuffer(t *testing.T) {
	for _, test := range sliceTests {
		result := test.in.Slice(test.start, test.len)
		assert.Equal(t, test.expected.Size(), result.Size())
		assert.Equal(t, test.expected.NumChannels(), result.NumChannels())
		for i := range test.expected {
			for j := 0; j < len(test.expected[i]); j++ {
				assert.Equal(t, test.expected[i][j], result[i][j])
			}
		}
	}
}

func TestSampleRate(t *testing.T) {
	sampleRate := phono.SampleRate(44100)
	expected := 500 * time.Millisecond
	result := sampleRate.DurationOf(22050)
	assert.Equal(t, expected, result)
}
