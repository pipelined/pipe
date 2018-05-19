package mixer

import (
	"context"

	"github.com/dudk/phono"
)

// Mixer summs up multiple channels of messages into a single channel
type Mixer struct {
	ins []<-chan *phono.Message

	process chan *input

	samples map[uint64][]*phono.Message
}

type input struct {
	c uint64
	*phono.Message
}

const processBuffer = 256

// New returns a new mixer
func New() *Mixer {
	return &Mixer{
		ins:     make([]<-chan *phono.Message, 0),
		process: make(chan *input, processBuffer),
		samples: make(map[uint64][]*phono.Message),
	}
}

// Pump returns a pump function which allows to read the out channel
func (m *Mixer) Pump() phono.PumpFunc {
	out := make(chan *phono.Message)
	return func(ctx context.Context, newMessage phono.NewMessageFunc) (<-chan *phono.Message, <-chan error, error) {
		errc := make(chan error, 1)
		go func() {
			defer close(out)
			defer close(m.process)
			defer close(errc)
			for {
				select {
				case <-ctx.Done():
					return
				case in := <-m.process:
					if _, ok := m.samples[in.c]; !ok {
						m.samples[in.c] = make([]*phono.Message, 0, len(m.ins))
					}
					m.samples[in.c] = append(m.samples[in.c], in.Message)
					// check if samples are ready for mix
					if len(m.samples[in.c]) == len(m.ins) {
						// TODO: mix and send message here
						out <- in.Message
						delete(m.samples, in.c)
					}
				}
			}
		}()
		return out, errc, nil
	}
}

// Sink adds a new input channel to mix
// this method should not be called after
func (m *Mixer) Sink() phono.SinkFunc {
	return func(ctx context.Context, in <-chan *phono.Message) (<-chan error, error) {
		m.ins = append(m.ins, in)
		errc := make(chan error, 1)
		var c uint64
		go func() {
			defer close(errc)
			for in != nil {
				select {
				case message, ok := <-in:
					if !ok {
						in = nil
					} else {
						c++
						m.process <- &input{c: c, Message: message}
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		return errc, nil
	}
}
