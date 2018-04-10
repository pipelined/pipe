package phono

import (
	"context"
	"fmt"
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
	// PublicOptionFunc represents an option which can be recieved by multiple objects
	publicOptionFunc func(OptionUser)
	// PrivateOptionFunc represents an option which can be recieved by one object
	privateOptionFunc func()
	// OptionValue is a type to wrap any option value
	OptionValue interface{}

	// OptionUser is an interface wich allows to Use options
	OptionUser interface {
		// Use consumes provided value
		Use(OptionValue)
		Validate() error
	}

	// Options represents current track attributes: time signature, bpm e.t.c.
	Options struct {
		public  map[string]publicOptionFunc
		private map[OptionUser]map[string]privateOptionFunc
	}
)

func NewOptions() *Options {
	return &Options{}
}

func (p *Options) Public(values ...OptionValue) *Options {
	if p.public == nil {
		p.public = make(map[string]publicOptionFunc)
	}

	newPublic := PublicOptions(values...)
	for t, v := range newPublic {
		p.public[t] = v
	}
	return p
}

func (p *Options) Private(ou OptionUser, values ...OptionValue) *Options {
	private, ok := p.private[ou]
	if !ok {
		private = make(map[string]privateOptionFunc)
	}
	newPrivate := PrivateOptions(ou, values...)
	for t, v := range newPrivate {
		private[t] = v
	}

	p.private[ou] = private
	return p
}

// ApplyTo consumes options defined for option user in this pulse
func (p Options) ApplyTo(ou OptionUser) {
	for _, option := range p.public {
		option(ou)
	}
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

// PublicOption creates a closure which can be used by any Configurable to apply it
func PublicOption(value OptionValue) publicOptionFunc {
	return func(ou OptionUser) {
		ou.Use(value)
	}
}

func PublicOptions(values ...OptionValue) map[string]publicOptionFunc {
	result := make(map[string]publicOptionFunc)
	for _, value := range values {
		t := fmt.Sprintf("%T", value)
		result[t] = PublicOption(value)
	}
	return result
}

// PrivateOption creates a closure which can be used by passed Configurable to apply it
func PrivateOption(ou OptionUser, value OptionValue) privateOptionFunc {
	return func() {
		ou.Use(value)
	}
}

func PrivateOptions(ou OptionUser, values ...OptionValue) map[string]privateOptionFunc {
	result := make(map[string]privateOptionFunc)
	for _, value := range values {
		t := fmt.Sprintf("%T", value)
		result[t] = PrivateOption(ou, value)
	}
	return result
}
