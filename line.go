// Package pipe provides functionality to build and execute DSP pipelines.
package pipe

import (
	"crypto/rand"
	"fmt"

	"pipelined.dev/pipe/internal/runner"
	"pipelined.dev/signal"
)

// pipeline components
type (
	PumpFunc func() (Pump, signal.SampleRate, int, error)
	// Pump is a source of samples. Pump method returns a new buffer with signal data.
	// If no data is available, io.EOF should be returned. If pump cannot provide data
	// to fulfill buffer, it can trim the size of the buffer to align it with actual data.
	// Buffer size can only be decreased.
	Pump struct {
		Pump func(out signal.Float64) error
		Hooks
	}

	ProcessorFunc func(signal.SampleRate, int) (Processor, signal.SampleRate, int, error)
	// Processor defines interface for pipe processors.
	// Processor should return output in the same signal buffer as input.
	// It is encouraged to implement in-place processing algorithms.
	// Buffer size could be changed during execution, but only decrease allowed.
	// Number of channels cannot be changed.
	Processor struct {
		Process func(in, out signal.Float64) error
		Hooks
	}

	SinkFunc func(signal.SampleRate, int) (Sink, error)
	// Sink is an interface for final stage in audio pipeline.
	// This components must not change buffer content. Line can have
	// multiple sinks and this will cause race condition.
	Sink struct {
		Sink func(in signal.Float64) error
		Hooks
	}

	Hook  func() error
	Hooks struct {
		Flush     Hook
		Interrupt Hook
		Reset     Hook
	}
)

// optional interfaces
type (
	// Resetter is a component that must be resetted before new run.
	// Reset hook is executed when Run happens.
	Resetter interface {
		Reset() error
	}

	// Interrupter is a component that has custom interruption logic.
	// Interrupt hook is executed when Cancel happens.
	Interrupter interface {
		Interrupt() error
	}

	// Flusher is a component that must be flushed in the end of execution.
	// Flush hook is executed in the end of the run. It will be skipped if Reset hook has failed.
	Flusher interface {
		Flush() error
	}
)

// newUID returns new unique id value.
func newUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x\n", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Line is a sound processing sequence of components.
// It has a single pump, zero or many processors executed sequentially
// and one or many sinks executed in parallel.
type Line struct {
	Pump       PumpFunc
	Processors []ProcessorFunc
	Sinks      []SinkFunc
}

// Processors is a helper function to use in line constructors.
func Processors(processors ...ProcessorFunc) []ProcessorFunc {
	return processors
}

// Sinks is a helper function to use in line constructors.
func Sinks(sinks ...SinkFunc) []SinkFunc {
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
