package phono

import (
	"context"

	"github.com/go-audio/audio"
)

// Message is an interface for pipe transport
type Message interface {
	// PutSamples assign samples to message
	PutSamples(samples [][]float64)
	// AsSamples represent message data as samples
	AsSamples() [][]float64
	// PutBuffer assign an audio.Buffer to message
	PutBuffer(buffer audio.Buffer, bufferLen int)
	// AsBuffer represent message data as audio.Buffer
	AsBuffer() audio.Buffer
	// BufferLen returns numChannels * bufferSize
	BufferLen() int
}

// Session is an interface for main container
type Session interface {
	NewMessage() Message
}

// PumpFunc is a function to pump sound data to pipe
type PumpFunc func(ctx context.Context) (out <-chan Message, errc <-chan error, err error)

// ProcessFunc is a function to process sound data in pipe
type ProcessFunc func(ctx context.Context, in <-chan Message) (out <-chan Message, errc <-chan error, err error)

// SinkFunc is a function to sink data from pipe
type SinkFunc func(ctx context.Context, in <-chan Message) (errc <-chan error, err error)
