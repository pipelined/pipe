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
	ErrorOnMake error
}

// Pump returns closure that creates new pumps.
func (m *Pump) Pump() pipe.PumpMaker {
	return func(bufferSize int) (pipe.Pump, pipe.Bus, error) {
		return pipe.Pump{
				Mutability: m.Mutability,
				Flush:      m.Flusher.Flush,
				Pump: func(s signal.Floating) (int, error) {
					if m.ErrorOnCall != nil {
						return 0, m.ErrorOnCall
					}
					if m.Counter.Samples == m.Limit {
						return 0, io.EOF
					}
					time.Sleep(m.Interval)

					read := s.Length()
					// ensure that we have enough samples
					if left := m.Limit - m.Counter.Samples; left < read {
						read = left
					}
					for i := 0; i < read*m.Channels; i++ {
						s.SetSample(i, m.Value)
					}
					m.Counter.advance(read)
					return read, nil
				},
			}, pipe.Bus{
				SampleRate: m.SampleRate,
				Channels:   m.Channels,
			},
			m.ErrorOnMake
	}
}

// Reset allows to reset pump.
func (m *Pump) Reset() mutability.Mutation {
	return m.Mutability.Mutate(
		func() error {
			m.Counter = Counter{}
			return nil
		})
}

// MockMutation mocks mutation, so errors can be simulated.
func (m *Mutator) MockMutation() mutability.Mutation {
	return m.Mutability.Mutate(
		func() error {
			m.Mutated = true
			return m.ErrorOnMutation
		})
}

// Processor are settings for pipe.Processor mock.
type Processor struct {
	Mutator
	Counter
	Flusher
	ErrorOnCall error
	ErrorOnMake error
}

// Processor returns closure that creates new processors.
func (m *Processor) Processor() pipe.ProcessorMaker {
	return func(bufferSize int, bus pipe.Bus) (pipe.Processor, pipe.Bus, error) {
		return pipe.Processor{
			Mutability: m.Mutator.Mutability,
			Flush:      m.Flusher.Flush,
			Process: func(in, out signal.Floating) error {
				if m.ErrorOnCall != nil {
					return m.ErrorOnCall
				}
				m.Counter.advance(signal.FloatingAsFloating(in, out))
				return nil
			},
		}, bus, m.ErrorOnMake
	}
}

// Sink are settings for pipe.Sink mock.
type Sink struct {
	Mutator
	Counter
	Flusher
	Discard     bool
	ErrorOnCall error
	ErrorOnMake error
}

// Sink returns closure that creates new sinks.
func (m *Sink) Sink() pipe.SinkMaker {
	return func(bufferSize int, b pipe.Bus) (pipe.Sink, error) {
		m.Counter.Values = signal.Allocator{Channels: b.Channels, Capacity: bufferSize}.Float64()
		return pipe.Sink{
			Mutability: m.Mutator.Mutability,
			Flush:      m.Flusher.Flush,
			Sink: func(in signal.Floating) error {
				if m.ErrorOnCall != nil {
					return m.ErrorOnCall
				}
				if !m.Discard {
					m.Counter.Values = m.Counter.Values.Append(in)
				}
				m.Counter.advance(in.Length())
				return nil
			},
		}, m.ErrorOnMake
	}
}
