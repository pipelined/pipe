package repeat

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"pipelined.dev/signal"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/pipe/pool"
)

type Repeater struct {
	mutable.Mutable
	bufferSize int
	sampleRate signal.SampleRate
	channels   int
	pumps      []chan message
}

type message struct {
	buffer signal.Floating
	pumps  int32
}

// Sink must be called once per broadcast.
func (r *Repeater) Sink() pipe.SinkMaker {
	return func(bufferSize int, bus pipe.Bus) (pipe.Sink, error) {
		r.sampleRate = bus.SampleRate
		r.channels = bus.Channels
		r.bufferSize = bufferSize
		p := pool.Get(signal.Allocator{
			Channels: bus.Channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		})
		return pipe.Sink{
			Mutable: r.Mutable,
			Sink: func(in signal.Floating) error {
				for _, pump := range r.pumps {
					out := p.GetFloat64()
					signal.FloatingAsFloating(in, out)
					pump <- message{
						pumps:  int32(len(r.pumps)),
						buffer: out,
					}
				}
				return nil
			},
			Flush: func(context.Context) error {
				for i := range r.pumps {
					close(r.pumps[i])
				}
				r.pumps = nil
				return nil
			},
		}, nil
	}
}

func (r *Repeater) AddLine(p pipe.Pipe, route pipe.Route) mutable.Mutation {
	return r.Mutable.Mutate(func() error {
		route.Pump = r.Pump()
		line, err := route.Line(r.bufferSize)
		if err != nil {
			return fmt.Errorf("error binding route: %w", err)
		}
		p.Push(p.AddLine(line))
		return nil
	})
}

// Pump must be called at least once per broadcast.
func (r *Repeater) Pump() pipe.PumpMaker {
	return func(bufferSize int) (pipe.Pump, pipe.Bus, error) {
		pump := make(chan message, 1)
		r.pumps = append(r.pumps, pump)
		var (
			message message
			ok      bool
		)
		p := pool.Get(signal.Allocator{
			Channels: r.channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		})
		return pipe.Pump{
				Pump: func(b signal.Floating) (int, error) {
					message, ok = <-pump
					if !ok {
						return 0, io.EOF
					}
					read := signal.FloatingAsFloating(message.buffer, b)
					if atomic.AddInt32(&message.pumps, -1) == 0 {
						p.PutFloat64(message.buffer)
					}
					return read, nil
				},
			},
			pipe.Bus{
				SampleRate: r.sampleRate,
				Channels:   r.channels,
			},
			nil
	}
}
