package phono_test

import (
	"sync"
	"testing"
	"time"

	"github.com/pipelined/phono"
	"github.com/stretchr/testify/assert"
)

func TestBuffer(t *testing.T) {
	var s phono.Buffer
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

func TestSliceBuffer(t *testing.T) {
	var sliceTests = []struct {
		in       phono.Buffer
		start    int
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
	sampleRate := 44100
	expected := 500 * time.Millisecond
	result := phono.DurationOf(sampleRate, 22050)
	assert.Equal(t, expected, result)
}

func TestSingleUse(t *testing.T) {
	var once sync.Once
	err := phono.SingleUse(&once)
	assert.Nil(t, err)
	err = phono.SingleUse(&once)
	assert.Equal(t, phono.ErrSingleUseReused, err)
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
		assert.Equal(t, c.expected, phono.DurationOf(c.sampleRate, c.samples))
	}
}

func TestReadInts(t *testing.T) {
	tests := []struct {
		label       string
		ints        []int
		NumChannels int
		BufferSize  int
		expected    phono.Buffer
	}{
		{
			label:       "Simple case",
			ints:        []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
			NumChannels: 2,
			BufferSize:  8,
			expected: phono.Buffer([][]float64{
				[]float64{1, 1, 1, 1, 1, 1, 1, 1},
				[]float64{2, 2, 2, 2, 2, 2, 2, 2},
			}),
		},
	}
	for _, test := range tests {
		b := phono.EmptyBuffer(test.NumChannels, test.BufferSize)
		b.ReadInts(test.ints)
		assert.Equal(t, test.expected.NumChannels(), b.NumChannels(), test.label)
		assert.Equal(t, test.expected.Size(), b.Size(), test.label)
		for i := range test.expected {
			for j := range test.expected[i] {
				assert.Equal(t, test.expected[i][j]/0x8000, b[i][j], test.label)
			}
		}
	}
}

func TestReadInts16(t *testing.T) {
	tests := []struct {
		label       string
		ints        []int16
		NumChannels int
		BufferSize  int
		expected    phono.Buffer
	}{
		{
			label:       "Simple case",
			ints:        []int16{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
			NumChannels: 2,
			BufferSize:  8,
			expected: phono.Buffer([][]float64{
				[]float64{1, 1, 1, 1, 1, 1, 1, 1},
				[]float64{2, 2, 2, 2, 2, 2, 2, 2},
			}),
		},
	}
	for _, test := range tests {
		b := phono.EmptyBuffer(test.NumChannels, test.BufferSize)
		b.ReadInts16(test.ints)
		assert.Equal(t, test.expected.NumChannels(), b.NumChannels(), test.label)
		assert.Equal(t, test.expected.Size(), b.Size(), test.label)
		for i := range test.expected {
			for j := range test.expected[i] {
				assert.Equal(t, test.expected[i][j]/0x8000, b[i][j], test.label)
			}
		}
	}
}

func TestInts(t *testing.T) {
	tests := []struct {
		phono.Buffer
		expected []int
	}{
		{
			Buffer: phono.Buffer([][]float64{
				[]float64{1, 1, 1, 1, 1, 1, 1, 1},
				[]float64{2, 2, 2, 2, 2, 2, 2, 2},
			}),
			expected: []int{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 1, 2},
		},
	}

	for _, test := range tests {
		ints := test.Buffer.Ints()
		for i := range test.expected {
			assert.Equal(t, test.expected[i]*0x7fff, ints[i])
		}
	}
}
