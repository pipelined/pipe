package mock

import (
	"context"
	"fmt"

	"github.com/dudk/phono"
)

const (
	defaultBufferSize = 512
	defaultSampleRate = 44100

	pumpSimpleConstraint      = 100
	processorSimpleConstraint = 0
	sinkSimpleConstraint      = -100
)

// Option types
type (
	// SimpleOption represents int64-valued option
	SimpleOption int64
	// ComplexOption represents  key-valued struct option
	ComplexOption struct {
		Key   string
		Value interface{}
	}

	// OptionUser is a simple option-user
	OptionUser struct {
		Simple  SimpleOption
		Complex ComplexOption
	}
	// SimpleOptionUser is a simple option-user with multiple values
	SimpleOptionUser struct {
		Simple1 SimpleOption
		Simple2 SimpleOption
	}
)

// Validate implements a phono.OptionUser interface
func (ou *OptionUser) Validate() error {
	return nil
}

// WithSimple assignes a SimpleOption to an OptionUser
func (ou *OptionUser) WithSimple(v SimpleOption) phono.OptionFunc {
	return func() {
		ou.Simple = v
	}
}

// WithComplex assignes a ComplexOption to an OptionUser
func (ou *OptionUser) WithComplex(v ComplexOption) phono.OptionFunc {
	return func() {
		ou.Complex = v
	}
}

// Pump mocks a pipe.Pump interface
type Pump struct {
	simple     SimpleOption
	newMessage phono.NewMessageFunc
}

// Pump implements pipe.Pump interface
func (p *Pump) Pump() phono.PumpFunc {
	return func(ctx context.Context, newMessage phono.NewMessageFunc) (<-chan phono.Message, <-chan error, error) {
		out := make(chan phono.Message)
		errc := make(chan error, 1)
		go func() {
			defer close(out)
			defer close(errc)
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				// case op, ok := <-options:
				// 	if !ok {
				// 		options = nil
				// 	}
				// 	op.ApplyTo(p)
				default:
					message := newMessage()
					out <- message
				}
			}
		}()
		return out, errc, nil
	}
}

// Validate implements phono.OptionUser
func (p *Pump) Validate() error {
	if p.simple > pumpSimpleConstraint {
		return fmt.Errorf("Simple bigger than %v", pumpSimpleConstraint)
	}
	return nil
}

// Processor mocks a pipe.Processor interface
type Processor struct {
	Simple SimpleOption
}

// Process implements pipe.Processor
func (p *Processor) Process() phono.ProcessFunc {
	// p.pulse = pulse
	// p.plugin.SetCallback(p.callback())
	return func(ctx context.Context, in <-chan phono.Message) (<-chan phono.Message, <-chan error, error) {
		errc := make(chan error, 1)
		out := make(chan phono.Message)
		go func() {
			defer close(out)
			defer close(errc)
			for in != nil {
				select {
				case m, ok := <-in:
					if !ok {
						in = nil
					} else {
						if m.Options != nil {
							m.Options.ApplyTo(p)
						}
						// m.Samples = p.plugin.Process(m.Samples)
						out <- m
					}
				case <-ctx.Done():
					return
				}
			}
		}()
		return out, errc, nil
	}
}

// Validate the processor
func (p *Processor) Validate() error {
	if p.Simple == processorSimpleConstraint {
		return fmt.Errorf("Simple equals to %v", processorSimpleConstraint)
	}
	return nil
}

// Sink mocks up a pipe.Sink interface
type Sink struct {
	simple SimpleOption
}

// Sink implements Sink interface
func (s *Sink) Sink() phono.SinkFunc {
	return func(ctx context.Context, in <-chan phono.Message) (<-chan error, error) {
		errc := make(chan error, 1)
		go func() {
			defer close(errc)
			for in != nil {
				select {
				case m, ok := <-in:
					if !ok {
						in = nil
					} else {
						if m.Options != nil {
							m.Options.ApplyTo(s)
						}
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		return errc, nil
	}
}

// Validate the sink
func (s *Sink) Validate() error {
	if s.simple < sinkSimpleConstraint {
		return fmt.Errorf("Simple less than %v", sinkSimpleConstraint)
	}
	return nil
}
