// Package signal provides an API to manipulate digital signals. It allows to:
// 	- convert interleaved data to non-interleaved
//	- convert bit depth for int signals
package signal

import (
	"math"
)

// Float64 is a non-interleaved float64 signal.
type Float64 [][]float64

// InterFloat64 is an interleaved float64 signal.
type InterFloat64 struct {
	Data        []float64
	NumChannels int
}

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

// AsFloat64 converts interleaved int signal to float64.
func (ints InterInt) AsFloat64() [][]float64 {
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
