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
func (f *Flusher) Flush() error {
	f.Flushed = true
	return f.ErrorOnFlush
}

// Source are settings for pipe.Source mock.
type Source struct {
	Mutator
	Counter
	Flusher
	Interval    time.Duration
	Limit       int
	Value       float64
	Channels    int
	SampleRate  signal.Frequency
	ErrorOnCall error
	ErrorOnMake error
}

// Source returns SourceAllocatorFunc.
func (m *Source) Source() pipe.SourceAllocatorFunc {
	return func(ctx context.Context, bufferSize int) (pipe.Source, error) {
		return pipe.Source{
				Mutability: m.Mutability,
				Output: pipe.SignalProperties{
					SampleRate: m.SampleRate,
					Channels:   m.Channels,
				},
				FlushFunc: m.Flusher.Flush,
				SourceFunc: func(s signal.Floating) (int, error) {
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
			},
			m.ErrorOnMake
	}
}

// Reset allows to reset source.
func (m *Source) Reset() mutability.Mutation {
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
func (m *Processor) Processor() pipe.ProcessorAllocatorFunc {
	return func(ctx context.Context, bufferSize int, props pipe.SignalProperties) (pipe.Processor, error) {
		return pipe.Processor{
			Mutability: m.Mutator.Mutability,
			Output:     props,
			FlushFunc:  m.Flusher.Flush,
			ProcessFunc: func(in, out signal.Floating) error {
				if m.ErrorOnCall != nil {
					return m.ErrorOnCall
				}
				m.Counter.advance(signal.FloatingAsFloating(in, out))
				return nil
			},
		}, m.ErrorOnMake
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
func (m *Sink) Sink() pipe.SinkAllocatorFunc {
	return func(ctx context.Context, bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		if !m.Discard {
			m.Counter.Values = signal.Allocator{Channels: props.Channels, Capacity: bufferSize}.Float64()
		}
		return pipe.Sink{
			Mutability: m.Mutator.Mutability,
			FlushFunc:  m.Flusher.Flush,
			SinkFunc: func(in signal.Floating) error {
				if m.ErrorOnCall != nil {
					return m.ErrorOnCall
				}
				if !m.Discard {
					m.Counter.Values.Append(in)
				}
				m.Counter.advance(in.Length())
				return nil
			},
		}, m.ErrorOnMake
	}
}
