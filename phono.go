package phono

import (
	"sync"
	"time"
)

// Pump is a source of samples
type Pump interface {
	Identifiable
	Pump(string) (PumpFunc, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	Identifiable
	Process(string) (ProcessFunc, error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Identifiable
	// RunSink(sourceID string) SinkRunner
	Sink(string) (SinkFunc, error)
}

type PumpFunc func() (Buffer, error)
type ProcessFunc func(Buffer) (Buffer, error)
type SinkFunc func(Buffer) error

// Pipe transport types
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
	// Identifiable defines a unique component
	Identifiable interface {
		ID() string
		SetID(string)
	}

	// UID is a string unique identifier
	UID struct {
		value string
	}

	// ParamFunc represents a function which mutates the pipe element (e.g. Pump, Processor or Sink)
	ParamFunc func()

	// Param is a structure for delayed parameters apply
	// used as return type in functions which enable Params support for different packages
	Param struct {
		ID     string
		Apply  ParamFunc
		at     int64
		atTime time.Duration
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
	// Tempo represents a tempo value
	Tempo float32
)

// At assignes param to sample position
func (p *Param) At(s int64) *Param {
	if p != nil {
		p.at = s
	}
	return p
}

// AtTime assignes param to time position
func (p *Param) AtTime(d time.Duration) *Param {
	if p != nil {
		p.atTime = d
	}
	return p
}

// ID of the pipe
func (i *UID) ID() string {
	return i.value
}

// SetID to the pipe
func (i *UID) SetID(id string) {
	i.value = id
}

// ReceivedBy returns channel which is closed when param received by identified entity
func ReceivedBy(wg *sync.WaitGroup, id Identifiable) Param {
	return Param{
		ID: id.ID(),
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
func (b Buffer) Clip(start int64, len int) *Clip {
	if b == nil || BufferSize(start) >= b.Size() || start < 0 {
		return nil
	}
	end := BufferSize(start + int64(len))
	if end >= b.Size() {
		len = int(b.Size()) - int(start)
	}
	return &Clip{
		Buffer: b,
		Start:  start,
		Len:    len,
	}
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
