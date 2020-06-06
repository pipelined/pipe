// Package mock provides mocks for pipeline components and allows to execute integration tests.
package mock

import (
	"context"
	"io"
	"time"

	"pipelined.dev/signal"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutability"
)

type (
	// Counter counts messages, samples and can capture sinked values.
	Counter struct {
		Messages int
		Samples  int
		Values   signal.Floating
	}

	// Flusher allows to mock components hooks.
	Flusher struct {
		Flushed      bool
		ErrorOnFlush error
	}

	// Mutator allows to mock mutations.
	Mutator struct {
		mutability.Mutability
		Mutated         bool
		ErrorOnMutation error
	}
)

// advance counter's metrics.
func (c *Counter) advance(size int) {
	c.Messages++
	c.Samples = c.Samples + size
}

// Flush implements pipe.Flusher.
func (f *Flusher) Flush(ctx context.Context) error {
	f.Flushed = true
	return f.ErrorOnFlush
}

// Pump are settings for pipe.Pump mock.
type Pump struct {
	Mutator
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
				Mutability: p.Mutability,
				Flush:      p.Flusher.Flush,
				Pump: func(s signal.Floating) (int, error) {
					if p.ErrorOnCall != nil {
						return 0, p.ErrorOnCall
					}
					if p.Counter.Samples == p.Limit {
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
func (p *Pump) Reset() mutability.Mutation {
	return p.Mutability.Mutate(
		func() error {
			p.Counter = Counter{}
			return nil
		})
}

// MockMutation mocks mutation, so errors can be simulated.
func (p *Mutator) MockMutation() mutability.Mutation {
	return p.Mutability.Mutate(
		func() error {
			p.Mutated = true
			return p.ErrorOnMutation
		})
}

// Processor are settings for pipe.Processor mock.
type Processor struct {
	Mutator
	Counter
	Flusher
	ErrorOnCall error
}

// Processor returns closure that creates new processors.
func (p *Processor) Processor() pipe.ProcessorMaker {
	return func(bufferSize int, bus pipe.Bus) (pipe.Processor, pipe.Bus, error) {
		return pipe.Processor{
			Mutability: p.Mutator.Mutability,
			Flush:      p.Flusher.Flush,
			Process: func(in, out signal.Floating) error {
				if p.ErrorOnCall != nil {
					return p.ErrorOnCall
				}
				p.Counter.advance(signal.FloatingAsFloating(in, out))
				return nil
			},
		}, bus, nil
	}
}

// Sink are settings for pipe.Sink mock.
type Sink struct {
	Mutator
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
			Mutability: sink.Mutator.Mutability,
			Flush:      sink.Flusher.Flush,
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
