package phono

import (
	"context"
)

// Pipe is a sound processing pipeline
type Pipe interface {
	Validate() error
	Run(ctx context.Context) error
}

// Message is an interface for pipe transport
type Message interface {
	// SetSamples assign samples to message
	SetSamples(samples [][]float64)
	// AsSamples represent message data as samples
	Samples() [][]float64
	// SetPulse to message
	SetPulse(Pulse)
	// Pulse returns current state
	Pulse() Pulse
	// BufferSize returns buffer size
	BufferSize() int
	// NumChannels number of channels
	NumChannels() int
}

// Session is an interface for main container
type Session interface {
	BufferSize() int
	SampleRate() int
	Pulse() Pulse
}

// Track represents a sequence of pipes
type Track interface {
	Pulse() Pulse
}

// Pulse represents current track attributes: time signature, bpm e.t.c.
type Pulse interface {
	NewMessage() Message
	Tempo() float64
	TimeSignature() (int, int)
	BufferSize() int
	SampleRate() int
	NumChannels() int
}

// PumpFunc is a function to pump sound data to pipe
type PumpFunc func(context.Context, <-chan Pulse) (out <-chan Message, errc <-chan error, err error)

// ProcessFunc is a function to process sound data in pipe
type ProcessFunc func(ctx context.Context, in <-chan Message) (out <-chan Message, errc <-chan error, err error)

// SinkFunc is a function to sink data from pipe
type SinkFunc func(ctx context.Context, in <-chan Message) (errc <-chan error, err error)

// SamplePosition represents current sample position
type SamplePosition uint64
