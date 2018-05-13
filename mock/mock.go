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

	// PumpMaxInterval is 2 seconds
	PumpMaxInterval = 2000
	// PumpDefaultLimit is 10 messages
	PumpDefaultLimit = 10
)

// Param types
type (
	// Interval between messages in ms
	Interval int
	// IntervalConsumer represents Interval parameter consumer
	IntervalConsumer interface {
		IntervalParam(Interval) phono.Param
	}
	// Limit messages
	Limit int
	// LimitConsumer represents Limit parameter consumer
	LimitConsumer interface {
		LimitParam(Limit) phono.Param
	}
)

// Pump mocks a pipe.Pump interface
type Pump struct {
	Interval
	Limit
	newMessage phono.NewMessageFunc
}

// IntervalParam implements IntervalConsumer
func (p *Pump) IntervalParam(i Interval) phono.Param {
	return phono.Param{
		Consumer: p,
		Apply: func() {
			p.Interval = i
		},
	}
}

// LimitParam implements LimitConsumer
func (p *Pump) LimitParam(l Limit) phono.Param {
	return phono.Param{
		Consumer: p,
		Apply: func() {
			p.Limit = l
		},
	}
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
	if p.Interval > PumpMaxInterval {
		return fmt.Errorf("Interval bigger than %v", PumpMaxInterval)
	}
	return nil
}

// Processor mocks a pipe.Processor interface
type Processor struct{}

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
	return nil
}

// Sink mocks up a pipe.Sink interface
type Sink struct{}

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
	return nil
}
