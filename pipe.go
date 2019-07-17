package pipe

import (
	"github.com/rs/xid"
)

// Pump is a source of samples. Pump method returns a new buffer with signal data.
// Implentetions should use next error conventions:
// 		- nil if a full buffer was read;
// 		- io.EOF if no data was read;
// 		- io.ErrUnexpectedEOF if not a full buffer was read.
// The latest case means that pump executed as expected, but not enough data was available.
// This incomplete buffer still will be sent further and pump will be finished gracefully.
// If no data was read or any other error was met, buffer should be nil.
type Pump interface {
	Pump(pipeID string) (func(bufferSize int) ([][]float64, error), int, int, error)
}

// Processor defines interface for pipe-processors
type Processor interface {
	Process(pipeID string, sampleRate, numChannels int) (func([][]float64) ([][]float64, error), error)
}

// Sink is an interface for final stage in audio pipeline
type Sink interface {
	Sink(pipeID string, sampleRate, numChannels int) (func([][]float64) error, error)
}

// newUID returns new unique id value.
func newUID() string {
	return xid.New().String()
}

// Pipe is a pipeline with fully defined sound processing sequence
// it has:
//	 1 		pump
//	 0..n 	processors
//	 1..n	sinks
type Pipe struct {
	Pump
	Processors []Processor
	Sinks      []Sink
}

// Processors is a helper function to use in pipe constructors.
func Processors(processors ...Processor) []Processor {
	return processors
}

// Sinks is a helper function to use in pipe constructors.
func Sinks(sinks ...Sink) []Sink {
	return sinks
}

// Wait for state transition or first error to occur.
func Wait(d chan error) error {
	for err := range d {
		if err != nil {
			return err
		}
	}
	return nil
}
