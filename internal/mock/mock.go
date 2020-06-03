// Package mock provides mocks for pipeline components and allows to execute integration tests.
package mock

import (
	"context"
	"io"
	"time"

	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"

	"pipelined.dev/pipe"
)

// Counter counts messages, samples and can capture sinked values.
type Counter struct {
	Messages int
	Samples  int
	Values   signal.Floating
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
	mutable.Mutable
	Counter
	Flusher
	Interval    time.Duration
	Limit       int
	Value       float64
	Channels    int
	SampleRate  signal.SampleRate
	ErrorOnCall error
}

// Pump returns closure that creates new pumps.
func (p *Pump) Pump() pipe.PumpMaker {
	return func(bufferSize int) (pipe.Pump, pipe.Bus, error) {
		return pipe.Pump{
				Mutable: p.Mutable,
				Flush:   p.Flusher.Flush,
				Pump: func(s signal.Floating) (int, error) {
					if p.ErrorOnCall != nil {
						return 0, p.ErrorOnCall
					}

					if p.Counter.Samples >= p.Limit {
						return 0, io.EOF
					}
					time.Sleep(p.Interval)

					read := s.Length()
					// ensure that we have enough samples
					if left := p.Limit - p.Counter.Samples; left < read {
						read = left
					}
					for i := 0; i < read*p.Channels; i++ {
						s.SetSample(i, p.Value)
					}
					p.Counter.advance(read)
					return read, nil
				},
			}, pipe.Bus{
				SampleRate: p.SampleRate,
				Channels:   p.Channels,
			}, nil
	}
}

// Reset allows to reset pump.
func (p *Pump) Reset() mutable.Mutation {
	return p.Mutable.Mutate(
		func() error {
			p.Counter = Counter{}
			return nil
		})
}

// Processor are settings for pipe.Processor mock.
type Processor struct {
	Counter
	Flusher
	ErrorOnCall error
}

// Processor returns closure that creates new processors.
func (processor *Processor) Processor() pipe.ProcessorMaker {
	return func(bufferSize int, bus pipe.Bus) (pipe.Processor, pipe.Bus, error) {
		return pipe.Processor{
			Flush: processor.Flusher.Flush,
			Process: func(in, out signal.Floating) error {
				if processor.ErrorOnCall != nil {
					return processor.ErrorOnCall
				}
				processor.Counter.advance(signal.FloatingAsFloating(in, out))
				return nil
			},
		}, bus, nil
	}
}

// Sink are settings for pipe.Sink mock.
type Sink struct {
	Counter
	Flusher
	Discard     bool
	ErrorOnCall error
}

// Sink returns closure that creates new sinks.
func (sink *Sink) Sink() pipe.SinkMaker {
	return func(bufferSize int, b pipe.Bus) (pipe.Sink, error) {
		sink.Counter.Values = signal.Allocator{Channels: b.Channels, Capacity: bufferSize}.Float64()
		return pipe.Sink{
			Flush: sink.Flusher.Flush,
			Sink: func(in signal.Floating) error {
				if sink.ErrorOnCall != nil {
					return sink.ErrorOnCall
				}
				if !sink.Discard {
					sink.Counter.Values = sink.Counter.Values.Append(in)
				}
				sink.Counter.advance(in.Length())
				return nil
			},
		}, nil
	}
}
