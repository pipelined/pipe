// Package signal provides an API to manipulate digital signals. It allows to:
// 	- convert interleaved data to non-interleaved
//	- convert bit depth for int signals
package signal

import (
	"math"
	"time"
)

// Float64 is a non-interleaved float64 signal.
type Float64 [][]float64

// Int is a non-interleaved int signal.
type Int [][]int

const (
	// BitDepth8 is 8 bit depth.
	BitDepth8 = BitDepth(8)
	// BitDepth16 is 16 bit depth.
	BitDepth16 = BitDepth(16)
	// BitDepth32 is 32 bit depth.
	BitDepth32 = BitDepth(32)
)

// InterInt is an interleaved int signal.
type InterInt struct {
	Data        []int
	NumChannels int
	BitDepth
}

// BitDepth contains values required for int-to-float and backward conversion.
type BitDepth int

// devider is used when int to float conversion is done.
func (bitDepth BitDepth) devider() int {
	switch bitDepth {
	case BitDepth8:
		return math.MaxInt8
	case BitDepth16:
		return math.MaxInt16
	case BitDepth32:
		return math.MaxInt32
	default:
		return 1
	}
}

// devider is used when float to int conversion is done.
func (bitDepth BitDepth) multiplier() int {
	switch bitDepth {
	case BitDepth8:
		return math.MaxInt8 - 1
	case BitDepth16:
		return math.MaxInt16 - 1
	case BitDepth32:
		return math.MaxInt32 - 1
	default:
		return 1
	}
}

// DurationOf returns time duration of passed samples for this sample rate.
func DurationOf(sampleRate int, samples int64) time.Duration {
	return time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
}

// AsFloat64 converts interleaved int signal to float64.
func (ints InterInt) AsFloat64() Float64 {
	if ints.Data == nil || ints.NumChannels == 0 {
		return nil
	}
	floats := make([][]float64, ints.NumChannels)
	bufSize := int(math.Ceil(float64(len(ints.Data)) / float64(ints.NumChannels)))

	// determine the devider for bit depth conversion
	devider := float64(ints.BitDepth.devider())

	for i := range floats {
		floats[i] = make([]float64, bufSize)
		pos := 0
		for j := i; j < len(ints.Data); j = j + ints.NumChannels {
			floats[i][pos] = float64(ints.Data[j]) / devider
			pos++
		}
	}
	return floats
}

// AsInterInt converts float64 signal to interleaved int.
func (floats Float64) AsInterInt(bitDepth BitDepth) []int {
	var numChannels int
	if numChannels = len(floats); numChannels == 0 {
		return nil
	}

	// determine the multiplier for bit depth conversion
	multiplier := float64(bitDepth.multiplier())

	ints := make([]int, len(floats[0])*numChannels)

	for j := range floats {
		for i := range floats[j] {
			ints[i*numChannels+j] = int(floats[j][i] * multiplier)
		}
	}
	return ints
}

// EmptyFloat64 returns an empty buffer of specified dimentions.
func EmptyFloat64(numChannels int, bufferSize int) Float64 {
	result := make([][]float64, numChannels)
	for i := range result {
		result[i] = make([]float64, bufferSize)
	}
	return result
}

// NumChannels returns number of channels in this sample slice
func (floats Float64) NumChannels() int {
	return len(floats)
}

// Size returns number of samples in single block in this sample slice
func (floats Float64) Size() int {
	if floats.NumChannels() == 0 {
		return 0
	}
	return len(floats[0])
}

// Append buffers set to existing one one
// new buffer is returned if b is nil
func (floats Float64) Append(source Float64) Float64 {
	if floats == nil {
		floats = make([][]float64, source.NumChannels())
		for i := range floats {
			floats[i] = make([]float64, 0, source.Size())
		}
	}
	for i := range source {
		floats[i] = append(floats[i], source[i]...)
	}
	return floats
}

// Slice creates a new copy of buffer from start position with defined legth
// if buffer doesn't have enough samples - shorten block is returned
//
// if start >= buffer size, nil is returned
// if start + len >= buffer size, len is decreased till the end of slice
// if start < 0, nil is returned
func (floats Float64) Slice(start int, len int) Float64 {
	if floats == nil || start >= floats.Size() || start < 0 {
		return nil
	}
	end := start + len
	result := make([][]float64, floats.NumChannels())
	for i := range floats {
		if end > floats.Size() {
			end = floats.Size()
		}
		result[i] = append(result[i], floats[i][start:end]...)
	}
	return result
}
