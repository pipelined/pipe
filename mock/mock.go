package mock

import (
	"context"
	"sync"
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

	// Counter can be used to check metrics for mocks
	Counter struct {
		m        sync.Mutex
		messages uint64
		samples  uint64
	}
)

func (c *Counter) advance(msg *phono.Message) {
	c.m.Lock()
	c.messages++
	c.samples = c.samples + uint64(msg.Buffer.Size())
	c.m.Unlock()
}

func (c *Counter) reset() {
	c.m.Lock()
	defer c.m.Unlock()
	c.messages, c.samples = 0, 0
}

// Count returns message and samples counted
func (c *Counter) Count() (uint64, uint64) {
	c.m.Lock()
	defer c.m.Unlock()
	return c.messages, c.samples
}

// Pump mocks a pipe.Pump interface
type Pump struct {
	phono.UID
	Counter
	Interval
	Limit
	phono.BufferSize
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
	p.Counter.reset()
	return func(ctx context.Context, newMessage phono.NewMessageFunc) (<-chan *phono.Message, <-chan error, error) {
		out := make(chan *phono.Message)
		errc := make(chan error, 1)
		go func() {
			defer close(out)
			defer close(errc)
			i := 0
			for i < int(p.Limit) {
				select {
				case <-ctx.Done():
					return
				default:
					message := newMessage()
					message.ApplyTo(p)
					message.Buffer = phono.Buffer([][]float64{make([]float64, p.BufferSize)})
					p.Counter.advance(message)
					out <- message
					i++
					time.Sleep(time.Millisecond * time.Duration(p.Interval))
				}
			}
		}()
		return out, errc, nil
	}
}

// Processor mocks a pipe.Processor interface
type Processor struct {
	phono.UID
	Counter
}

// Process implements pipe.Processor
func (p *Processor) Process() phono.ProcessFunc {
	p.Counter.reset()
	return func(in <-chan *phono.Message) (<-chan *phono.Message, <-chan error, error) {
		errc := make(chan error, 1)
		out := make(chan *phono.Message)
		go func() {
			defer close(out)
			defer close(errc)
			for in != nil {
				select {
				case m, ok := <-in:
					if !ok {
						return
					}
					p.Counter.advance(m)
					if m.Params != nil {
						m.Params.ApplyTo(p)
					}
					out <- m
				}
			}
		}()
		return out, errc, nil
	}
}

// Sink mocks up a pipe.Sink interface
// Buffer is not thread-safe, so should not be checked while pipe is running
type Sink struct {
	phono.UID
	Counter
	phono.Buffer
}

// Sink implements Sink interface
func (s *Sink) Sink() phono.SinkFunc {
	s.Counter.reset()
	return func(in <-chan *phono.Message) (<-chan error, error) {
		errc := make(chan error, 1)
		go func() {
			defer close(errc)
			for in != nil {
				select {
				case m, ok := <-in:
					if !ok {
						return
					}
					s.Buffer.Append(m.Buffer)
					s.Counter.advance(m)
					m.RecievedBy(s)
					if m.Params != nil {
						m.Params.ApplyTo(s)
					}
				}
			}
		}()

		return errc, nil
	}
}
