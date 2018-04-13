package phono

import (
	"context"
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
	}

	// NewMessageFunc is a message-producer function
	NewMessageFunc func(*Options) Message
)

// Pipe function types
type (
	// PumpFunc is a function to pump sound data to pipe
	PumpFunc func(context.Context, <-chan Options) (out <-chan Message, errc <-chan error, err error)

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
)

// NewOptions returns a new Options instance with initialised map inside
func NewOptions() *Options {
	return &Options{
		private: make(map[OptionUser][]OptionFunc),
	}
}

// Add accepts an option-user and
func (p *Options) Add(reciever OptionUser, options ...OptionFunc) *Options {
	private, ok := p.private[reciever]
	if !ok {
		private = make([]OptionFunc, 0, len(options))
	}
	private = append(private, options...)

	p.private[reciever] = private
	return p
}

// ApplyTo consumes options defined for option user in this pulse
func (p Options) ApplyTo(ou OptionUser) {
	if options, ok := p.private[ou]; ok {
		for _, option := range options {
			option()
		}
	}
}

// NewMessage returns a default message producer which caches options
// if new options are passed - next message will contain them
func (p PumpFunc) NewMessage() NewMessageFunc {
	var options *Options
	return func(newOptions *Options) Message {
		if newOptions != options {
			options = newOptions
			return Message{Options: newOptions}
		}
		return Message{}
	}
}
