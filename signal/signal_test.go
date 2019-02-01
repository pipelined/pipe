package signal_test

import (
	"math"
	"testing"

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
