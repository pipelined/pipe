// Package mock provides mocks for pipeline components and allows to execute integration tests.
package mock

import (
	"context"
	"io"
	"time"

	"pipelined.dev/pipe"

	"pipelined.dev/signal"
)

// Counter counts messages, samples and can capture sinked values.
type Counter struct {
	Messages int
	Samples  int
	Values   signal.Float64
}

// advance counter's metrics.
func (c *Counter) advance(size int) {
	c.Messages++
	c.Samples = c.Samples + size
}

// Flusher allows to mock components hooks.
type Flusher struct {
	Flushed      bool
	ErrorOnFlush error
}

// Flush implements pipe.Flusher.
func (f *Flusher) Flush(ctx context.Context) error {
	f.Flushed = true
	return f.ErrorOnFlush
}

// Pump are settings for pipe.Pump mock.
type Pump struct {
	Counter
	Flusher
	Interval    time.Duration
	Limit       int
	Value       float64
	NumChannels int
	SampleRate  signal.SampleRate
	ErrorOnCall error
}

// Pump returns closure with mocked pump.
func (p *Pump) Pump() pipe.PumpMaker {
	return func(bufferSize int) (pipe.Pump, pipe.Bus, error) {
		return pipe.Pump{
				Flush: p.Flusher.Flush,
				Pump: func(b signal.Float64) error {
					if p.ErrorOnCall != nil {
						return p.ErrorOnCall
					}

					if p.Counter.Samples >= p.Limit {
						return io.EOF
					}
					time.Sleep(p.Interval)

					// calculate buffer size.
					bs := b.Size()
					// check if we need a shorter.
					if left := p.Limit - p.Counter.Samples; left < bs {
						bs = left
					}
					for i := range b {
						// resize buffer
						b[i] = b[i][:bs]
						for j := range b[i] {
							b[i][j] = p.Value
						}
					}
					p.Counter.advance(bs)
					return nil
				},
			},
			pipe.Bus{
				BufferSize:  bufferSize,
				SampleRate:  p.SampleRate,
				NumChannels: p.NumChannels,
			},
			nil
	}
}

// Reset allows to reset pump.
func (p *Pump) Reset() func() error {
	return func() error {
		p.Counter = Counter{}
		return nil
	}
}

// Processor are settings for pipe.Processor mock.
type Processor struct {
	Counter
	Flusher
	ErrorOnCall error
}

// Processor returns closure with mocked processor.
func (processor *Processor) Processor() pipe.ProcessorMaker {
	return func(bus pipe.Bus) (pipe.Processor, pipe.Bus, error) {
		return pipe.Processor{
			Flush: processor.Flusher.Flush,
			Process: func(in, out signal.Float64) error {
				if processor.ErrorOnCall != nil {
					return processor.ErrorOnCall
				}
				processor.Counter.advance(in.Size())
				for i := range in {
					copy(out[i], in[i])
				}
				return nil
			},
		}, bus, nil
	}
}

// Sink are settings for pipe.Processor mock.
type Sink struct {
	Counter
	Flusher
	Discard     bool
	ErrorOnCall error
}

// Sink returns closure with mocked processor.
func (sink *Sink) Sink() pipe.SinkMaker {
	return func(pipe.Bus) (pipe.Sink, error) {
		return pipe.Sink{
			Flush: sink.Flusher.Flush,
			Sink: func(in signal.Float64) error {
				if sink.ErrorOnCall != nil {
					return sink.ErrorOnCall
				}
				if !sink.Discard {
					sink.Counter.Values = sink.Counter.Values.Append(in)
				}
				sink.Counter.advance(in.Size())
				return nil
			},
		}, nil
	}
}
