package phono

import (
	"context"
	"sync"
)

// Pipe transport types
type (
	// Samples represent a sample data sliced per channel
	Samples [][]float64

	// Message is a main structure for pipe transport
	Message struct {
		// Samples of message
		Samples
		// Pulse
		*Options
		*sync.WaitGroup
	}

	// NewMessageFunc is a message-producer function
	NewMessageFunc func() Message
)

// Pipe function types
type (
	// PumpFunc is a function to pump sound data to pipe
	PumpFunc func(context.Context, NewMessageFunc) (out <-chan Message, errc <-chan error, err error)

	// ProcessFunc is a function to process sound data in pipe
	ProcessFunc func(ctx context.Context, in <-chan Message) (out <-chan Message, errc <-chan error, err error)

	// SinkFunc is a function to sink data from pipe
	SinkFunc func(ctx context.Context, in <-chan Message) (errc <-chan error, err error)
)

// Options support types
type (
	// OptionFunc represents a function which applies the option
	OptionFunc func()

	// OptionUser is an interface wich allows to Use options
	OptionUser interface {
		Validate() error
	}

	// Options represents current track attributes: time signature, bpm e.t.c.
	Options struct {
		private map[OptionUser][]OptionFunc
	}
)

// Small types for common options
type (
	// BufferSize represents a buffer size value
	BufferSize int
	// NumChannels represents a number of channels
	NumChannels int
	// SampleRate represents a sample rate value
	SampleRate int
	// Tempo represents a tempo value
	Tempo float32
	// TimeSignature represents a time signature
	TimeSignature struct {
		NotesPerBar int // 3 in 3/4
		NoteValue   int // 4 in 3/4
	}
	// SamplePosition represents a position in samples measure
	SamplePosition int64
)

// NewOptions returns a new Options instance with initialised map inside
func NewOptions() *Options {
	return &Options{
		private: make(map[OptionUser][]OptionFunc),
	}
}

// AddOptionsFor accepts an option-user and options
func (op *Options) AddOptionsFor(reciever OptionUser, options ...OptionFunc) *Options {
	private, ok := op.private[reciever]
	if !ok {
		private = make([]OptionFunc, 0, len(options))
	}
	private = append(private, options...)

	op.private[reciever] = private
	return op
}

// ApplyTo consumes options defined for option user in this pulse
func (op *Options) ApplyTo(ou OptionUser) {
	if op == nil {
		return
	}
	if options, ok := op.private[ou]; ok {
		for _, option := range options {
			option()
		}
	}
}

// RecievedBy should be called by sink once message is recieved
func (m *Message) RecievedBy(reciever interface{}) {
	// TODO check in map of receivers
	if m.WaitGroup != nil {
		m.Done()
	}
}
