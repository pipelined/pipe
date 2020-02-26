package repeat

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
	"pipelined.dev/signal/pool"
)

type Repeater struct {
	pipe.Mutability
	bufferSize  int
	sampleRate  signal.SampleRate
	numChannels int
	pumps       []chan message
	pool        *pool.Pool
}

type message struct {
	buffer *signal.Float64
	pumps  int32
}

// Sink must be called once per broadcast.
func (r *Repeater) Sink() pipe.SinkMaker {
	return func(bus pipe.Bus) (pipe.Sink, error) {
		r.bufferSize = bus.BufferSize
		r.sampleRate = bus.SampleRate
		r.numChannels = bus.NumChannels
		r.pool = pool.New(r.numChannels, bus.BufferSize)
		var buffer *signal.Float64
		return pipe.Sink{
			Mutability: r.Mutability,
			Sink: func(b signal.Float64) error {
				buffer = r.pool.Alloc()
				copyFloat64(*buffer, b)
				for _, pump := range r.pumps {
					pump <- message{
						pumps:  int32(len(r.pumps)),
						buffer: buffer,
					}
				}
				return nil
			},
			Flush: func(context.Context) error {
				for _, pump := range r.pumps {
					close(pump)
				}
				return nil
			},
		}, nil
	}
}

func (r *Repeater) AddLine(p pipe.Pipe, line pipe.Line) pipe.Mutation {
	return r.Mutability.Mutate(func() error {
		line.Pump = r.Pump()
		route, err := line.Route(r.bufferSize)
		if err != nil {
			return fmt.Errorf("error binding route: %w", err)
		}
		p.Push(p.AddRoute(route))
		return nil
	})
}

// Pump must be called at least once per broadcast.
func (r *Repeater) Pump() pipe.PumpMaker {
	pump := make(chan message, 1)
	r.pumps = append(r.pumps, pump)
	return func(bufferSize int) (pipe.Pump, pipe.Bus, error) {
		var (
			message message
			ok      bool
		)
		return pipe.Pump{
				Pump: func(b signal.Float64) error {
					message, ok = <-pump
					if !ok {
						return io.EOF
					}
					copyFloat64(b, *message.buffer)
					if atomic.AddInt32(&message.pumps, -1) == 0 {
						r.pool.Free(message.buffer)
					}
					return nil
				},
			},
			pipe.Bus{
				BufferSize:  bufferSize,
				SampleRate:  r.sampleRate,
				NumChannels: r.numChannels,
			},
			nil
	}
}

func copyFloat64(d signal.Float64, r signal.Float64) {
	for i := range r {
		copy(d[i], r[i])
	}
}
