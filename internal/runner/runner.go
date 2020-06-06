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
	Signal   signal.Floating      // Buffer of message.
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
		// flush on return
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errs <- fmt.Errorf("error flushing pump: %w", err)
			}
		}()
		var (
			read      int
			mutations mutability.Mutations
			outSignal signal.Floating
			err       error
		)
		for {
			select {
			case mutations = <-mutationsChan:
			case <-ctx.Done():
				return
			default:
			}

			if err = mutations.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating pump: %w", err)
				return
			}

			outSignal = r.Output.GetFloat64()
			if read, err = r.Fn(outSignal); err != nil {
				if err != io.EOF {
					errs <- fmt.Errorf("error running pump: %w", err)
				}
				// this buffer wasn't sent, free now
				r.Output.PutFloat64(outSignal)
				return
			}
			if read != outSignal.Length() {
				outSignal = outSignal.Slice(0, read)
			}
			meter(outSignal.Length())

			select {
			case out <- Message{Mutators: mutations, Signal: outSignal}:
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
		// flush on return
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errs <- fmt.Errorf("error flushing processor: %w", err)
			}
		}()
		var (
			message   Message
			outSignal signal.Floating
			ok        bool
			err       error
		)
		for {
			select {
			case message, ok = <-in:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			if err = message.Mutators.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating processor: %w", err)
				r.Input.PutFloat64(message.Signal)
				return
			}

			outSignal = r.Output.GetFloat64()
			err = r.Fn(message.Signal, outSignal)
			r.Input.PutFloat64(message.Signal)
			if err != nil {
				errs <- fmt.Errorf("error running processor: %w", err)
				// this buffer wasn't sent, free now
				r.Output.PutFloat64(outSignal)
				return
			}
			meter(outSignal.Length())

			select {
			case out <- Message{Mutators: message.Mutators, Signal: outSignal}:
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
		// flush on return
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errs <- fmt.Errorf("error flushing sink: %w", err)
			}
		}()
		var (
			message Message
			ok      bool
			err     error
		)
		for {
			// receive new message
			select {
			case message, ok = <-in:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			// apply Mutators
			if err = message.Mutators.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating sink: %w", err)
				r.Input.PutFloat64(message.Signal) // need to free
				return
			}
			err = r.Fn(message.Signal)     // sink a buffer
			meter(message.Signal.Length()) // capture metrics
			r.Input.PutFloat64(message.Signal)
			if err != nil {
				errs <- fmt.Errorf("error running sink: %w", err)
				return
			}
		}
	}()

	return errs
}
