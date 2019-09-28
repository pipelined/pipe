// Package pipe provides functionality to build and execute DSP pipelines.
// Examples could be found in [examples repository](https://github.com/pipelined/example).
package pipe

import (
	"crypto/rand"
	"fmt"

	"github.com/pipelined/pipe/internal/runner"
)

// pipeline components
type (
	// Pump is a source of samples. Pump method returns a new buffer with signal data.
	// Implentetions should use next error conventions:
	// 		- nil if a full buffer was read;
	// 		- io.EOF if no data was read;
	// 		- io.ErrUnexpectedEOF if not a full buffer was read.
	// The latest case means that pump executed as expected, but not enough data was available.
	// This incomplete buffer still will be sent further and pump will be finished gracefully.
	// If no data was read or any other error was met, buffer should be nil.
	Pump interface {
		Pump(pipeID string) (func(bufferSize int) ([][]float64, error), int, int, error)
	}

	// Processor defines interface for pipe processors
	Processor interface {
		Process(pipeID string, sampleRate, numChannels int) (func([][]float64) ([][]float64, error), error)
	}

	// Sink is an interface for final stage in audio pipeline
	Sink interface {
		Sink(pipeID string, sampleRate, numChannels int) (func([][]float64) error, error)
	}
)

// optional interfaces
type (
	// Resetter is a component that must be resetted before new run.
	// Reset hook is executed when Run happens.
	Resetter interface {
		Reset(string) error
	}

	// Interrupter is a component that has custom interruption logic.
	// Interrupt hook is executed when Cancel happens.
	Interrupter interface {
		Interrupt(string) error
	}

	// Flusher is a component that must be flushed in the end of execution.
	// Flush hook is executed in the end of the run. It will be skipped if Reset hook has failed.
	Flusher interface {
		Flush(string) error
	}
)

// newUID returns new unique id value.
func newUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x\n", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
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
func Wait(d <-chan error) error {
	for err := range d {
		if err != nil {
			return err
		}
	}
	return nil
}

// BindHooks of component.
func BindHooks(v interface{}) runner.Hooks {
	return runner.Hooks{
		Flush:     flusher(v),
		Interrupt: interrupter(v),
		Reset:     resetter(v),
	}
}

// flusher checks if interface implements Flusher and if so, return it.
func flusher(i interface{}) runner.Hook {
	if v, ok := i.(Flusher); ok {
		return v.Flush
	}
	return nil
}

// flusher checks if interface implements Flusher and if so, return it.
func interrupter(i interface{}) runner.Hook {
	if v, ok := i.(Interrupter); ok {
		return v.Interrupt
	}
	return nil
}

// flusher checks if interface implements Flusher and if so, return it.
func resetter(i interface{}) runner.Hook {
	if v, ok := i.(Resetter); ok {
		return v.Reset
	}
	return nil
}
