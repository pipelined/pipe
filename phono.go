package phono

import "github.com/go-audio/audio"

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
