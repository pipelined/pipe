package runner

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutability"
)

// Message is a main structure for pipe transport
type Message struct {
	Signal   signal.Floating   // Buffer of message.
	Mutators mutability.Mutations // Mutators for pipe.
}

type (
	// Pump executes pipe.Pump components.
	Pump struct {
		Mutability [16]byte
		Flush
		Output *signal.Pool
		Fn     func(out signal.Floating) (int, error)
		Meter  metric.ResetFunc
	}

	// Processor executes pipe.Processor components.
	Processor struct {
		Mutability [16]byte
		Flush
		Input  *signal.Pool
		Output *signal.Pool
		Fn     func(in, out signal.Floating) error
		Meter  metric.ResetFunc
	}

	// Sink executes pipe.Sink components.
	Sink struct {
		Mutability [16]byte
		Flush
		Input *signal.Pool
		Fn    func(in signal.Floating) error
		Meter metric.ResetFunc
	}
)

// Flush is a closure that triggers pipe component flush function.
type Flush func(context.Context) error

func (fn Flush) call(ctx context.Context) error {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

// Run starts the Pump runner.
func (r Pump) Run(ctx context.Context, mutationsChan chan mutability.Mutations) (<-chan Message, <-chan error) {
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
			read      int
			err       error
			mutations mutability.Mutations
			output    signal.Floating
		)
		for {
			// receive new message
			select {
			case mutations = <-mutationsChan:
			case <-ctx.Done():
				return
			default:
			}
			// apply Mutators
			if err = mutations.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating pump: %w", err)
				return
			}

			// allocate new buffer
			output = r.Output.GetFloat64()
			// pump new buffer
			if read, err = r.Fn(output); err != nil {
				if err != io.EOF {
					errs <- fmt.Errorf("error running pump: %w", err)
				}
				return
			}
			if read != output.Length() {
				output = output.Slice(0, read)
			}
			meter(output.Length()) // capture metrics

			// push message further
			select {
			case out <- Message{Mutators: mutations, Signal: output}:
				mutations = nil
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
			output signal.Floating
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
			if err = m.Mutators.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating processor: %w", err)
				r.Input.PutFloat64(m.Signal) // need to free
				return
			}
			output = r.Output.GetFloat64()
			err = r.Fn(m.Signal, output) // process new buffer
			r.Input.PutFloat64(m.Signal) // put buffer back to the input pool
			if err != nil {
				errs <- fmt.Errorf("error running processor: %w", err)
				return
			}
			meter(output.Length()) // capture metrics

			// send message further
			select {
			case out <- Message{Mutators: m.Mutators, Signal: output}:
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
			if err = m.Mutators.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating sink: %w", err)
				r.Input.PutFloat64(m.Signal) // need to free
				return
			}
			err = r.Fn(m.Signal)     // sink a buffer
			meter(m.Signal.Length()) // capture metrics
			r.Input.PutFloat64(m.Signal)
			if err != nil {
				errs <- fmt.Errorf("error running sink: %w", err)
				return
			}
		}
	}()

	return errs
}
