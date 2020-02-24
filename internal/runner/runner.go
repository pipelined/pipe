package runner

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutate"
)

// Pool provides pooling of signal.Float64 buffers.
type Pool interface {
	Alloc() *signal.Float64
	Free(*signal.Float64)
}

// Message is a main structure for pipe transport
type Message struct {
	Buffer   *signal.Float64 // Buffer of message.
	Mutators mutate.Mutators // Mutators for pipe.
}

type Bus struct {
	signal.SampleRate
	NumChannels int
	Pool
}

type (
	// Pump executes pipe.Pump components.
	Pump struct {
		mutate.Receiver
		Flush
		Output Bus
		Fn     func(out signal.Float64) error
		Meter  metric.ResetFunc
	}

	// Processor executes pipe.Processor components.
	Processor struct {
		mutate.Receiver
		Flush
		Input  Bus
		Output Bus
		Fn     func(in, out signal.Float64) error
		Meter  metric.ResetFunc
	}

	// Sink executes pipe.Sink components.
	Sink struct {
		mutate.Receiver
		Flush
		Input Bus
		Fn    func(in signal.Float64) error
		Meter metric.ResetFunc
	}
)

type Flush func(context.Context) error

func (fn Flush) call(ctx context.Context) error {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

// Run starts the Pump runner.
func (r Pump) Run(ctx context.Context, give chan<- chan mutate.Mutators, take chan mutate.Mutators) (<-chan Message, <-chan error) {
	out := make(chan Message, 1)
	errs := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errs)
		// Flush hook on return
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errs <- fmt.Errorf("error flushing pump: %w", err)
			}
		}()
		var (
			err      error
			mutators mutate.Mutators
			output   *signal.Float64
		)
		for {
			// request new message
			select {
			case give <- take:
			case <-ctx.Done():
				return
			}

			// receive new message
			select {
			case mutators = <-take:
			case <-ctx.Done():
				return
			}
			// apply Mutators
			if err = mutators.ApplyTo(r.Receiver); err != nil {
				errs <- fmt.Errorf("error mutating pump: %w", err)
				return
			}

			// allocate new buffer
			output = r.Output.Alloc()
			// pump new buffer
			if err = r.Fn(*output); err != nil {
				if err != io.EOF {
					errs <- fmt.Errorf("error running pump: %w", err)
				}
				return
			}
			meter(output.Size()) // capture metrics

			// push message further
			select {
			case out <- Message{Mutators: mutators, Buffer: output}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errs
}

// Run starts the Processor runner.
func (r Processor) Run(ctx context.Context, in <-chan Message) (<-chan Message, <-chan error) {
	errs := make(chan error, 1)
	out := make(chan Message, 1)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errs)
		// Flush hook on return
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errs <- fmt.Errorf("error flushing processor: %w", err)
			}
		}()
		var (
			err    error
			m      Message
			ok     bool
			output *signal.Float64
		)
		for {
			// retrieve new message
			select {
			case m, ok = <-in:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			// apply Mutators
			if err = m.Mutators.ApplyTo(r.Receiver); err != nil {
				errs <- fmt.Errorf("error mutating processor: %w", err)
				r.Input.Free(m.Buffer) // need to free
				return
			}
			output = r.Output.Alloc()
			err = r.Fn(*m.Buffer, *output) // process new buffer
			r.Input.Free(m.Buffer)         // put buffer back to the input pool
			if err != nil {
				errs <- fmt.Errorf("error running processor: %w", err)
				return
			}
			meter(output.Size()) // capture metrics

			// send message further
			select {
			case out <- Message{Mutators: m.Mutators, Buffer: output}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errs
}

// Run starts the sink runner.
func (r Sink) Run(ctx context.Context, in <-chan Message) <-chan error {
	errs := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(errs)
		// Flush hook on return
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errs <- fmt.Errorf("error flushing sink: %w", err)
			}
		}()
		var (
			err error
			m   Message
			ok  bool
		)
		for {
			// receive new message
			select {
			case m, ok = <-in:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			// apply Mutators
			if err = m.Mutators.ApplyTo(r.Receiver); err != nil {
				errs <- fmt.Errorf("error mutating sink: %w", err)
				r.Input.Free(m.Buffer) // need to free
				return
			}
			err = r.Fn(*m.Buffer)  // sink a buffer
			meter(m.Buffer.Size()) // capture metrics
			r.Input.Free(m.Buffer)
			if err != nil {
				errs <- fmt.Errorf("error running sink: %w", err)
				return
			}
		}
	}()

	return errs
}
