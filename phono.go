package phono

import (
	"errors"
	"sync"
)

// Pump is a source of samples. Pump method returns a new buffer with signal data.
// Implentetions should use next error conventions:
// 		- nil if a full buffer was read;
// 		- io.EOF if no data was read;
// 		- io.ErrUnexpectedEOF if not a full buffer was read.
// The latest case means that pump executed as expected, but not enough data was available.
// This incomplete buffer still will be sent further and pump will be finished gracefully.
type Pump interface {
	Pump(pipeID string, bufferSize int) (func() ([][]float64, error), int, int, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	Process(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) ([][]float64, error), error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error)
}

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
