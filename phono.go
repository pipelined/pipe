package phono

import (
	"context"
	"fmt"

	"github.com/go-audio/audio"
)

// Message is an interface for pipe transport
type Message interface {
	// PutSamples assign samples to message
	PutSamples(samples [][]float64)
	// AsSamples represent message data as samples
	Samples() [][]float64
	// BufferLen returns numChannels * bufferSize
	BufferLen() int
}

// Session is an interface for main container
type Session interface {
	NewMessage() Message
	BufferSize() int
	SampleRate() int
}

// PumpFunc is a function to pump sound data to pipe
type PumpFunc func(ctx context.Context) (out <-chan Message, errc <-chan error, err error)

// ProcessFunc is a function to process sound data in pipe
type ProcessFunc func(ctx context.Context, in <-chan Message) (out <-chan Message, errc <-chan error, err error)

// SinkFunc is a function to sink data from pipe
type SinkFunc func(ctx context.Context, in <-chan Message) (errc <-chan error, err error)

// AsSamples converts from audio.Buffer to [][]float64 samples
func AsSamples(b audio.Buffer) ([][]float64, error) {
	if b == nil {
		return nil, nil
	}

	if b.PCMFormat() == nil {
		return nil, fmt.Errorf("Format for Buffer is not defined")
	}

	numChannels := b.PCMFormat().NumChannels
	s := make([][]float64, numChannels)
	bufferLen := numChannels * b.NumFrames()

	switch b.(type) {
	case *audio.IntBuffer:
		ib := b.(*audio.IntBuffer)
		for i := range s {
			s[i] = make([]float64, 0, b.NumFrames())
			for j := i; j < bufferLen; j = j + numChannels {
				s[i] = append(s[i], float64(ib.Data[j])/0x8000)
			}
		}
		return s, nil
	default:
		return nil, fmt.Errorf("Conversion to [][]float64 from %T is not defined", b)
	}
}

// AsBuffer converts from [][]float64 to audio.Buffer
func AsBuffer(b audio.Buffer, s [][]float64) error {
	if b == nil || s == nil {
		return nil
	}

	numChannels := len(s)
	bufferLen := numChannels * len(s[0])

	switch b.(type) {
	case *audio.IntBuffer:
		ib := b.(*audio.IntBuffer)
		ib.Data = make([]int, bufferLen)
		for i := range s[0] {
			for j := range s {
				ib.Data[i*numChannels+j] = int(s[j][i] * 0x7fff)
			}
		}
		return nil
	default:
		return fmt.Errorf("Conversion to %T from [][]float64 is not defined", b)
	}
}
