package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/dudk/phono"
)

const (
	defaultBufferSize = 512
	defaultSampleRate = 44100

	pumpSimpleConstraint      = 100
	processorSimpleConstraint = 0
	sinkSimpleConstraint      = -100
	// PumpMaxInterval is 2 seconds
	PumpMaxInterval = 2000
)

// Param types
type (
	// SimpleParam represents int64-valued param
	SimpleParam int64
	// ComplexParam represents  key-valued struct param
	ComplexParam struct {
		Key   string
		Value interface{}
	}

	// ParamUser is a simple param-user
	ParamUser struct {
		Simple  SimpleParam
		Complex ComplexParam
	}
	// SimpleParamUser is a simple param-user with multiple values
	SimpleParamUser struct {
		Simple1 SimpleParam
		Simple2 SimpleParam
	}

	// Interval between messages in ms
	Interval int
	// IntervalConsumer represents Interval parameter consumer
	IntervalConsumer interface {
		IntervalParam(Interval) phono.Param
	}
	// Limit messages
	Limit int
)

// Validate implements a phono.ParamUser interface
func (ou *ParamUser) Validate() error {
	return nil
}

// WithSimple assignes a SimpleParam to an ParamUser
func (ou *ParamUser) WithSimple(v SimpleParam) phono.ParamFunc {
	return func() {
		ou.Simple = v
	}
}

// WithComplex assignes a ComplexParam to an ParamUser
func (ou *ParamUser) WithComplex(v ComplexParam) phono.ParamFunc {
	return func() {
		ou.Complex = v
	}
}

// Pump mocks a pipe.Pump interface
// TODO: add interval and number of messages as params
type Pump struct {
	Interval
	Limit
	newMessage phono.NewMessageFunc
}

// Pump implements pipe.Pump interface
func (p *Pump) Pump() phono.PumpFunc {
	fmt.Println("mock.Pump called")
	return func(ctx context.Context, newMessage phono.NewMessageFunc) (<-chan phono.Message, <-chan error, error) {
		fmt.Println("mock.Pump started")
		out := make(chan phono.Message)
		errc := make(chan error, 1)
		go func() {
			defer close(out)
			defer close(errc)
			for i := 0; i < int(p.Limit); i++ {
				select {
				case <-ctx.Done():
					fmt.Println("mock.Pump finished")
					return
				default:
					fmt.Println("mock.Pump request new message")
					message := newMessage()
					fmt.Println("mock.Pump got new message")
					out <- message
					time.Sleep(time.Millisecond * time.Duration(p.Interval))
				}
			}
		}()
		return out, errc, nil
	}
}

// Validate implements phono.ParamUser
func (p *Pump) Validate() error {
	if p.Interval > pumpSimpleConstraint {
		return fmt.Errorf("Simple bigger than %v", pumpSimpleConstraint)
	}
	return nil
}

// Processor mocks a pipe.Processor interface
type Processor struct {
	Simple SimpleParam
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
						if m.Params != nil {
							m.Params.ApplyTo(p)
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
	simple SimpleParam
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
						fmt.Printf("mock.Sink new message: %+v\n", m)
						m.RecievedBy(s)
						if m.Params != nil {
							m.Params.ApplyTo(s)
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
