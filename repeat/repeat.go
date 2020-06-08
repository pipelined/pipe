package repeat

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"pipelined.dev/signal"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutability"
	"pipelined.dev/pipe/pool"
)

// Repeater sinks the signal and sources it to multiple lines.
type Repeater struct {
	mutability.Mutability
	bufferSize int
	sampleRate signal.SampleRate
	channels   int
	sources    []chan message
}

type message struct {
	buffer  signal.Floating
	sources int32
}

// Sink must be called once per broadcast.
func (r *Repeater) Sink() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		r.sampleRate = props.SampleRate
		r.channels = props.Channels
		r.bufferSize = bufferSize
		p := pool.Get(signal.Allocator{
			Channels: props.Channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		})
		return pipe.Sink{
			Mutability: r.Mutability,
			SinkFunc: func(in signal.Floating) error {
				for _, source := range r.sources {
					out := p.GetFloat64()
					signal.FloatingAsFloating(in, out)
					source <- message{
						sources: int32(len(r.sources)),
						buffer:  out,
					}
				}
				return nil
			},
			FlushFunc: func(context.Context) error {
				for i := range r.sources {
					close(r.sources[i])
				}
				r.sources = nil
				return nil
			},
		}, nil
	}
}

// AddLine adds the line to the repeater.
func (r *Repeater) AddLine(p pipe.Pipe, route pipe.Route) mutability.Mutation {
	return r.Mutability.Mutate(func() error {
		route.Source = r.Source()
		line, err := route.Line(r.bufferSize)
		if err != nil {
			return fmt.Errorf("error binding route: %w", err)
		}
		p.Push(p.AddLine(line))
		return nil
	})
}

// Source must be called at least once per repeater.
func (r *Repeater) Source() pipe.SourceAllocatorFunc {
	return func(bufferSize int) (pipe.Source, pipe.SignalProperties, error) {
		source := make(chan message, 1)
		r.sources = append(r.sources, source)
		var (
			message message
			ok      bool
		)
		p := pool.Get(signal.Allocator{
			Channels: r.channels,
			Length:   bufferSize,
			Capacity: bufferSize,
		})
		return pipe.Source{
				SourceFunc: func(b signal.Floating) (int, error) {
					message, ok = <-source
					if !ok {
						return 0, io.EOF
					}
					read := signal.FloatingAsFloating(message.buffer, b)
					if atomic.AddInt32(&message.sources, -1) == 0 {
						p.PutFloat64(message.buffer)
					}
					return read, nil
				},
			},
			pipe.SignalProperties{
				SampleRate: r.sampleRate,
				Channels:   r.channels,
			},
			nil
	}
}
