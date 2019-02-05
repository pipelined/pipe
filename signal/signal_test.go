package signal_test

import (
	"math"
	"testing"
	"time"

	"github.com/pipelined/phono/signal"
	"github.com/stretchr/testify/assert"
)

func TestInterIntsAsFloat64(t *testing.T) {
	tests := []struct {
		ints        []int
		numChannels int
		bitDepth    signal.BitDepth
		expected    [][]float64
	}{
		{
			ints:        []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
			numChannels: 2,
			expected: [][]float64{
				[]float64{1, 1, 1, 1, 1, 1, 1, 1},
				[]float64{2, 2, 2, 2, 2, 2, 2, 2},
			},
		},
		{
			ints:        []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1},
			numChannels: 2,
			expected: [][]float64{
				[]float64{1, 1, 1, 1, 1, 1, 1, 1},
				[]float64{2, 2, 2, 2, 2, 2, 2, 0},
			},
		},
		{
			ints:        []int{math.MaxInt16, math.MaxInt16 * 2},
			numChannels: 2,
			expected: [][]float64{
				[]float64{1},
				[]float64{2},
			},
			bitDepth: signal.BitDepth16,
		},
		{
			ints:     nil,
			expected: nil,
		},
		{
			ints:     []int{1, 2, 3},
			expected: nil,
		},
		{
			ints:        []int{1, 2, 3, 4},
			numChannels: 5,
			expected: [][]float64{
				[]float64{1},
				[]float64{2},
				[]float64{3},
				[]float64{4},
				[]float64{0},
			},
		},
	}

	for _, test := range tests {
		ints := signal.InterInt{
			Data:        test.ints,
			NumChannels: test.numChannels,
			BitDepth:    test.bitDepth,
		}
		result := ints.AsFloat64()
		assert.Equal(t, len(test.expected), len(result))
		for i := range test.expected {
			for j, val := range test.expected[i] {
				assert.Equal(t, val, result[i][j])
			}
		}
	}
}

func TestFloat64AsInterInt(t *testing.T) {
	tests := []struct {
		floats   [][]float64
		bitDepth signal.BitDepth
		expected []int
	}{
		{
			floats: [][]float64{
				[]float64{1, 1, 1, 1, 1, 1, 1, 1},
				[]float64{2, 2, 2, 2, 2, 2, 2, 2},
			},
			expected: []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
		},
		{
			floats: [][]float64{
				[]float64{1, 1, 1, 1, 1, 1, 1, 1},
				[]float64{2, 2, 2, 2, 2, 2},
			},
			expected: []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 0, 1, 0},
		},
		{
			floats: [][]float64{
				[]float64{1},
				[]float64{2},
			},
			bitDepth: signal.BitDepth16,
			expected: []int{1 * (math.MaxInt16 - 1), 2 * (math.MaxInt16 - 1)},
		},
		{
			floats:   nil,
			expected: nil,
		},
		{
			floats:   [][]float64{},
			expected: nil,
		},
		{
			floats: [][]float64{
				[]float64{},
				[]float64{},
			},
			expected: []int{},
		},
		{
			floats: [][]float64{
				[]float64{1},
				[]float64{2},
				[]float64{3},
				[]float64{4},
				[]float64{5},
			},
			expected: []int{1, 2, 3, 4, 5},
		},
	}

	for _, test := range tests {
		floats := signal.Float64(test.floats)
		ints := floats.AsInterInt(test.bitDepth)
		assert.Equal(t, len(test.expected), len(ints))
		for i := range test.expected {
			assert.Equal(t, test.expected[i], ints[i])
		}
	}
}

func TestSliceFloat64(t *testing.T) {
	var sliceTests = []struct {
		in       signal.Float64
		start    int
		len      int
		expected signal.Float64
	}{
		{
			in:       signal.Float64([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
			start:    1,
			len:      2,
			expected: signal.Float64([][]float64{[]float64{1, 2}, []float64{1, 2}}),
		},
		{
			in:       signal.Float64([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
			start:    5,
			len:      2,
			expected: signal.Float64([][]float64{[]float64{5, 6}, []float64{5, 6}}),
		},
		{
			in:       signal.Float64([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
			start:    7,
			len:      4,
			expected: signal.Float64([][]float64{[]float64{7, 8, 9}, []float64{7, 8, 9}}),
		},
		{
			in:       signal.Float64([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
			start:    9,
			len:      1,
			expected: signal.Float64([][]float64{[]float64{9}}),
		},
		{
			in:       signal.Float64([][]float64{[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}),
			start:    10,
			len:      1,
			expected: nil,
		},
	}

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

func TestFloat64(t *testing.T) {
	var s signal.Float64
	assert.Equal(t, 0, s.NumChannels())
	assert.Equal(t, 0, s.Size())
	s = [][]float64{[]float64{}}
	assert.Equal(t, 1, s.NumChannels())
	assert.Equal(t, 0, s.Size())
	s[0] = make([]float64, 512)
	assert.Equal(t, 512, s.Size())

	s2 := [][]float64{make([]float64, 512)}
	s = s.Append(s2)
	assert.Equal(t, 1024, s.Size())
	s2[0] = make([]float64, 1024)
	s = s.Append(s2)
	assert.Equal(t, 2048, s.Size())
}

func TestDuration(t *testing.T) {
	var tests = []struct {
		sampleRate int
		samples    int64
		expected   time.Duration
	}{
		{
			sampleRate: 44100,
			samples:    44100,
			expected:   1 * time.Second,
		},
		{
			sampleRate: 44100,
			samples:    22050,
			expected:   500 * time.Millisecond,
		},
		{
			sampleRate: 44100,
			samples:    50,
			expected:   1133786 * time.Nanosecond,
		},
	}
	for _, c := range tests {
		assert.Equal(t, c.expected, signal.DurationOf(c.sampleRate, c.samples))
	}
}
