package split

import (
	"context"
	"io"
	"sync/atomic"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
	"pipelined.dev/signal/pool"
)

type Splitter struct {
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
func (s *Splitter) Sink() pipe.SinkFunc {
	return func(bufferSize int, sr signal.SampleRate, nc int) (pipe.Sink, error) {
		s.sampleRate = sr
		s.numChannels = nc
		s.pool = pool.New(s.numChannels, bufferSize)
		var buffer *signal.Float64
		return pipe.Sink{
			Sink: func(b signal.Float64) error {
				buffer = s.pool.Alloc()
				copyFloat64(*buffer, b)
				for _, pump := range s.pumps {
					pump <- message{
						pumps:  int32(len(s.pumps)),
						buffer: buffer,
					}
				}
				return nil
			},
			Flush: func(context.Context) error {
				for _, pump := range s.pumps {
					close(pump)
				}
				return nil
			},
		}, nil
	}
}

// Pump must be called at least once per broadcast.
func (s *Splitter) Pump() pipe.PumpFunc {
	pump := make(chan message, 1)
	s.pumps = append(s.pumps, pump)
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
					s.pool.Free(message.buffer)
				}
				return nil
			},
		}, s.sampleRate, s.numChannels, nil
	}
}

func copyFloat64(d signal.Float64, s signal.Float64) {
	for i := range s {
		copy(d[i], s[i])
	}
}
