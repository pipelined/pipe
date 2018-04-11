package phono

import (
	"context"
)

// Message is an interface for pipe transport
type Message struct {
	// Samples of message
	Samples
	// Pulse
	Options
}

// Samples represent a sample data sliced per channel
type Samples [][]float64

// Types for Options support
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

// PumpFunc is a function to pump sound data to pipe
type PumpFunc func(context.Context, <-chan Options) (out <-chan Message, errc <-chan error, err error)

// ProcessFunc is a function to process sound data in pipe
type ProcessFunc func(ctx context.Context, in <-chan Message) (out <-chan Message, errc <-chan error, err error)

// SinkFunc is a function to sink data from pipe
type SinkFunc func(ctx context.Context, in <-chan Message) (errc <-chan error, err error)
