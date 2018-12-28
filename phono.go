package phono

import (
	"errors"
	"sync"
	"time"

	"github.com/rs/xid"
)

// Pump is a source of samples
type Pump interface {
	ID() string
	Pump(string) (PumpFunc, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	ID() string
	Process(string) (ProcessFunc, error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	ID() string
	// RunSink(sourceID string) SinkRunner
	Sink(string) (SinkFunc, error)
}

// Components closure types.
type (
	// PumpFunc produces new buffer of data.
	PumpFunc func() (Buffer, error)

	// ProcessFunc consumes and returns new buffer of data.
	ProcessFunc func(Buffer) (Buffer, error)

	// SinkFunc consumes buffer of data.
	SinkFunc func(Buffer) error
)

type (
	// Buffer represent a sample data sliced per channel
	Buffer [][]float64

	// Clip represents a segment of buffer
	// Start is a start position of segment
	// Len is a length of segment
	//
	// Clip is not a copy of a buffer
	Clip struct {
		Buffer
		Start int64
		Len   int
	}
)

// Param-related types
type (
	// UID is a string unique identifier
	UID string

	// ParamFunc represents a function which mutates the pipe element (e.g. Pump, Processor or Sink)
	ParamFunc func()

	// Param is a structure for delayed parameters apply
	// used as return type in functions which enable Params support for different packages
	Param struct {
		ID     string        // id of mutable object.
		Apply  ParamFunc     // mutator.
		At     int64         // sample position of this param.
		AtTime time.Duration // time position of this param.
	}
)

// Generic types
type (
	// BufferSize represents a buffer size value
	BufferSize int
	// NumChannels represents a number of channels
	NumChannels int
	// SampleRate represents a sample rate value
	SampleRate int
)

// Metrics types.
type (
	// Metric stores meters of pipe components.
	Metric interface {
		Meter(id string, counters ...string) Meter
		Measure() Measure
	}

	// Meter stores counters values.
	Meter interface {
		Store(counter string, value interface{})
		Load(counter string) interface{}
	}

	// Measure is a snapshot of full metric with all counters.
	Measure map[string]map[string]interface{}
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
	// ErrEOP is returned if pump finished processing and indicates a gracefull ending.
	ErrEOP = errors.New("End of pipe")
	// ErrInterrupted is returned when pipe's execution was interrupted due to error.
	ErrInterrupted = errors.New("Pipe execution interrupted")
)

// NewUID returns new UID value.
func NewUID() UID {
	return UID(xid.New().String())
}

// ID returns string value of unique identifier. Should be used to satisfy Identifiable interface.
func (id UID) ID() string {
	return string(id)
}

// ReceivedBy returns channel which is closed when param received by identified entity
func ReceivedBy(wg *sync.WaitGroup, id string) Param {
	return Param{
		ID: id,
		Apply: func() {
			wg.Done()
		},
	}
}

// NumChannels returns number of channels in this sample slice
func (b Buffer) NumChannels() NumChannels {
	if b == nil {
		return 0
	}
	return NumChannels(len(b))
}

// Size returns number of samples in single block in this sample slice
func (b Buffer) Size() BufferSize {
	if b.NumChannels() == 0 {
		return 0
	}
	return BufferSize(len((b)[0]))
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
func (b Buffer) Slice(start int64, len int) Buffer {
	if b == nil || BufferSize(start) >= b.Size() || start < 0 {
		return nil
	}
	end := BufferSize(start + int64(len))
	result := Buffer(make([][]float64, b.NumChannels()))
	for i := range b {
		if end > b.Size() {
			end = b.Size()
		}
		result[i] = append(result[i], b[i][start:end]...)
	}
	return result
}

// Clip creates a new clip from asset with defined start and length
//
// if start >= buffer size, nil is returned
// if start + len >= buffer size, len is decreased till the end of slice
// if start < 0, nil is returned
func (b Buffer) Clip(start int64, len int) Clip {
	if b == nil || BufferSize(start) >= b.Size() || start < 0 {
		return Clip{}
	}
	end := BufferSize(start + int64(len))
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

// EmptyBuffer returns an empty buffer of specified length
func EmptyBuffer(numChannels NumChannels, bufferSize BufferSize) Buffer {
	result := Buffer(make([][]float64, numChannels))
	for i := range result {
		result[i] = make([]float64, bufferSize)
	}
	return result
}

// DurationOf returns time duration of passed samples for this sample rate.
func (s SampleRate) DurationOf(v int64) time.Duration {
	return time.Duration(float64(v) / float64(s) * float64(time.Second))
}
