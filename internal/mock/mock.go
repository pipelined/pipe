// Package mock provides mocks for pipeline components and allows to execute integration tests.
package mock

import (
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

// Hooks allows to mock components hooks.
type Hooks struct {
	Resetted    bool
	Flushed     bool
	Interrupted bool

	ErrorOnReset     error
	ErrorOnFlush     error
	ErrorOnInterrupt error
}

// Reset implements pipe.Resetter.
func (h *Hooks) Reset() error {
	h.Resetted = true
	return h.ErrorOnReset
}

// Interrupt implements pipe.Interrupter.
func (h *Hooks) Interrupt() error {
	h.Interrupted = true
	return h.ErrorOnInterrupt
}

// Flush implements pipe.Flusher.
func (h *Hooks) Flush() error {
	h.Flushed = true
	return h.ErrorOnFlush
}

// PumpOptions are settings for pipe.Pump mock.
type PumpOptions struct {
	Counter
	Hooks
	Interval    time.Duration
	Limit       int
	Value       float64
	NumChannels int
	SampleRate  signal.SampleRate
	ErrorOnCall error
}

// Pump returns closure with mocked pump.
func Pump(options *PumpOptions) pipe.PumpFunc {
	return func() (pipe.Pump, signal.SampleRate, int, error) {
		return pipe.Pump{
			Hooks: pipe.Hooks{
				Reset:     options.Hooks.Reset,
				Interrupt: options.Hooks.Interrupt,
				Flush:     options.Hooks.Flush,
			},
			Pump: func(b signal.Float64) error {
				if options.ErrorOnCall != nil {
					return options.ErrorOnCall
				}

				if options.Counter.Samples >= options.Limit {
					return io.EOF
				}
				time.Sleep(options.Interval)

				// calculate buffer size.
				bs := b.Size()
				// check if we need a shorter.
				if left := options.Limit - options.Counter.Samples; left < bs {
					bs = left
				}
				for i := range b {
					// resize buffer
					b[i] = b[i][:bs]
					for j := range b[i] {
						b[i][j] = options.Value
					}
				}
				options.Counter.advance(bs)
				return nil
			},
		}, options.SampleRate, options.NumChannels, nil
	}
}

// ProcessorOptions are settings for pipe.Processor mock.
type ProcessorOptions struct {
	Counter
	Hooks
	ErrorOnCall error
}

// Processor returns closure with mocked processor.
func Processor(options *ProcessorOptions) pipe.ProcessorFunc {
	return func(sampleRate signal.SampleRate, numChannels int) (pipe.Processor, signal.SampleRate, int, error) {
		return pipe.Processor{
			Hooks: pipe.Hooks{
				Reset:     options.Hooks.Reset,
				Interrupt: options.Hooks.Interrupt,
				Flush:     options.Hooks.Flush,
			},
			Process: func(in, out signal.Float64) error {
				if options.ErrorOnCall != nil {
					return options.ErrorOnCall
				}
				options.Counter.advance(in.Size())
				for i := range in {
					copy(out[i], in[i])
				}
				return nil
			},
		}, sampleRate, numChannels, nil
	}
}

// SinkOptions are settings for pipe.Processor mock.
type SinkOptions struct {
	Counter
	Hooks
	Discard     bool
	ErrorOnCall error
}

// Sink returns closure with mocked processor.
func Sink(options *SinkOptions) pipe.SinkFunc {
	return func(sampleRate signal.SampleRate, numChannels int) (pipe.Sink, error) {
		return pipe.Sink{
			Hooks: pipe.Hooks{
				Reset:     options.Hooks.Reset,
				Interrupt: options.Hooks.Interrupt,
				Flush:     options.Hooks.Flush,
			},
			Sink: func(in signal.Float64) error {
				if options.ErrorOnCall != nil {
					return options.ErrorOnCall
				}
				if !options.Discard {
					options.Counter.Values = options.Counter.Values.Append(in)
				}
				options.Counter.advance(in.Size())
				return nil
			},
		}, nil
	}
}
