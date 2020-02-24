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
	return func(bufferSize int, sr signal.SampleRate, nc int) (pipe.Sink, error) {
		r.bufferSize = bufferSize
		r.sampleRate = sr
		r.numChannels = nc
		r.pool = pool.New(r.numChannels, bufferSize)
		var buffer *signal.Float64
		return pipe.Sink{
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

func (r *Repeater) AddLine(p pipe.Pipe, line pipe.Line) func() error {
	return func() error {
		line.Pump = r.Pump()
		route, err := line.Route(r.bufferSize)
		if err != nil {
			return fmt.Errorf("error binding route: %w", err)
		}
		p.Push(p.AddRoute(route))
		return nil
	}
}

// Pump must be called at least once per broadcast.
func (r *Repeater) Pump() pipe.PumpMaker {
	pump := make(chan message, 1)
	r.pumps = append(r.pumps, pump)
	return func(bufferSize int) (pipe.Pump, signal.SampleRate, int, error) {
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
		}, r.sampleRate, r.numChannels, nil
	}
}

func copyFloat64(d signal.Float64, r signal.Float64) {
	for i := range r {
		copy(d[i], r[i])
	}
}
