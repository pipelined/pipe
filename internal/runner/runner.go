package runner

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutator"
)

// Pool provides pooling of signal.Float64 buffers.
type Pool interface {
	Alloc() *signal.Float64
	Free(*signal.Float64)
}

// Message is a main structure for pipe transport
type Message struct {
	Buffer   *signal.Float64  // Buffer of message.
	Mutators mutator.Mutators // Mutators for pipe.
}

type Bus struct {
	signal.SampleRate
	NumChannels int
	Pool
}

type (
	// Pump executes pipe.Pump components.
	Pump struct {
		*mutator.Receiver
		Flush
		Output Bus
		Fn     func(out signal.Float64) error
		Meter  metric.ResetFunc
	}

	// Processor executes pipe.Processor components.
	Processor struct {
		*mutator.Receiver
		Flush
		Input  Bus
		Output Bus
		Fn     func(in, out signal.Float64) error
		Meter  metric.ResetFunc
	}

	// Sink executes pipe.Sink components.
	Sink struct {
		*mutator.Receiver
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
func (r Pump) Run(ctx context.Context, give chan<- chan mutator.Mutators, take chan mutator.Mutators) (<-chan Message, <-chan error) {
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
			mutators mutator.Mutators
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
			mutators.ApplyTo(r.Receiver) // apply Mutators

			// allocate new buffer
			output = r.Output.Alloc()
			err = r.Fn(*output)  // pump new buffer
			meter(output.Size()) // capture metrics
			// handle error
			if err != nil {
				if err != io.EOF {
					errs <- fmt.Errorf("error running pump: %w", err)
				}
				return
			}

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

			m.Mutators.ApplyTo(r.Receiver) // apply Mutators
			output = r.Output.Alloc()
			err = r.Fn(*m.Buffer, *output) // process new buffer
			meter(output.Size())           // capture metrics
			r.Input.Free(m.Buffer)         // put buffer back to the input pool
			if err != nil {
				errs <- fmt.Errorf("error running processor: %w", err)
				return
			}

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
		var m Message
		var ok bool
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

			m.Mutators.ApplyTo(r.Receiver) // apply Mutators
			err := r.Fn(*m.Buffer)         // sink a buffer
			meter(m.Buffer.Size())         // capture metrics
			r.Input.Free(m.Buffer)
			if err != nil {
				errs <- fmt.Errorf("error running sink: %w", err)
				return
			}
		}
	}()

	return errs
}
