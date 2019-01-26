package phono

import (
	"errors"
	"sync"
	"time"
)

// Pump is a source of samples. Pump method returns a new buffer with signal data.
// Implentetions should use next error conventions:
// 		- nil if a full buffer was read;
// 		- io.EOF if no data was read;
// 		- io.ErrUnexpectedEOF if not a full buffer was read.
// The latest case means that pump executed as expected, but not enough data was available.
// This incomplete buffer still will be sent further and pump will be finished gracefully.
type Pump interface {
	Pump(string) (func() (Buffer, error), error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	Process(string) (func(Buffer) (Buffer, error), error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(string) (func(Buffer) error, error)
}

// Components closure types.
type (
// PumpFunc produces new buffer of data.
// PumpFunc func() (Buffer, error)

// ProcessFunc consumes and returns new buffer of data.
// ProcessFunc func(Buffer) (Buffer, error)

// SinkFunc consumes buffer of data.
// SinkFunc func(Buffer) error
)

type (
	// Buffer represent a sample data sliced per channel
	Buffer [][]float64

	// Clip represents a segment of buffer.
	//
	// Clip is not a copy of a buffer.
	Clip struct {
		Buffer
		Start int
		Len   int
	}
)

// SingleUse is designed to be used in runner-return functions to define a single-use pipe components.
func SingleUse(once *sync.Once) (err error) {
	err = ErrSingleUseReused
	once.Do(func() {
		err = nil
	})
	return
}

var (
	// ErrSingleUseReused is returned when object designed for single-use is being reused.
	ErrSingleUseReused = errors.New("Error reuse single-use object")
)

// ReceivedBy returns channel which is closed when param received by identified entity
func ReceivedBy(wg *sync.WaitGroup, id string) func() {
	return func() {
		wg.Done()
	}
}

// NumChannels returns number of channels in this sample slice
func (b Buffer) NumChannels() int {
	if b == nil {
		return 0
	}
	return len(b)
}

// Size returns number of samples in single block in this sample slice
func (b Buffer) Size() int {
	if b.NumChannels() == 0 {
		return 0
	}
	return len(b[0])
}

// Append buffers set to existing one one
// new buffer is returned if b is nil
func (b Buffer) Append(source Buffer) Buffer {
	if b == nil {
		b = make([][]float64, source.NumChannels())
		for i := range b {
			b[i] = make([]float64, 0, source.Size())
		}
	}
	for i := range source {
		b[i] = append(b[i], source[i]...)
	}
	return b
}

// Slice creates a new copy of buffer from start position with defined legth
// if buffer doesn't have enough samples - shorten block is returned
//
// if start >= buffer size, nil is returned
// if start + len >= buffer size, len is decreased till the end of slice
// if start < 0, nil is returned
func (b Buffer) Slice(start int, len int) Buffer {
	if b == nil || start >= b.Size() || start < 0 {
		return nil
	}
	end := start + len
	result := Buffer(make([][]float64, b.NumChannels()))
	for i := range b {
		if end > b.Size() {
			end = b.Size()
		}
		result[i] = append(result[i], b[i][start:end]...)
	}
	return result
}

// Clip creates a new clip from buffer with defined start and length.
//
// if start >= buffer size, nil is returned
// if start + len >= buffer size, len is decreased till the end of slice
// if start < 0, nil is returned
func (b Buffer) Clip(start int, len int) Clip {
	if b == nil || start >= b.Size() || start < 0 {
		return Clip{}
	}
	end := start + len
	if end >= b.Size() {
		len = int(b.Size()) - int(start)
	}
	return Clip{
		Buffer: b,
		Start:  start,
		Len:    len,
	}
}

// Ints converts [][]float64 buffer to []int of PCM data.
func (b Buffer) Ints() []int {
	if b == nil {
		return nil
	}
	numChannels := int(b.NumChannels())
	ints := make([]int, int(b.Size())*numChannels)
	for i := range b[0] {
		for j := range b {
			ints[i*numChannels+j] = int(b[j][i] * 0x7fff)
		}
	}
	return ints
}

// ReadInts converts PCM encoded as ints to float64 buffer.
func (b Buffer) ReadInts(ints []int) {
	if b == nil {
		return
	}
	intsLen := len(ints)
	numChannels := int(b.NumChannels())
	for i := range b {
		b[i] = make([]float64, 0, intsLen)
		for j := i; j < intsLen; j = j + numChannels {
			b[i] = append(b[i], float64(ints[j])/0x8000)
		}
	}
}

// ReadInts16 converts PCM encoded as ints16 to float64 buffer.
func (b Buffer) ReadInts16(ints []int16) {
	if b == nil {
		return
	}
	intsLen := len(ints)
	numChannels := int(b.NumChannels())
	for i := range b {
		b[i] = make([]float64, 0, intsLen)
		for j := i; j < intsLen; j = j + numChannels {
			b[i] = append(b[i], float64(ints[j])/0x8000)
		}
	}
}

// EmptyBuffer returns an empty buffer of specified length
func EmptyBuffer(numChannels int, bufferSize int) Buffer {
	result := Buffer(make([][]float64, numChannels))
	for i := range result {
		result[i] = make([]float64, bufferSize)
	}
	return result
}

// DurationOf returns time duration of passed samples for this sample rate.
func DurationOf(sampleRate int, samples int64) time.Duration {
	return time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
}
