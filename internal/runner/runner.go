package runner

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/mutability"
)

// Message is a main structure for pipe transport
type Message struct {
	Signal               signal.Floating // Buffer of message.
	mutability.Mutations                 // Mutators for pipe.
}

type (
	// Runner defines the sequence of executors.
	Runner struct {
		Mutators chan mutability.Mutations
		Source
		Processors []Processor
		Sink
	}

	// Source executes pipe.Source components.
	Source struct {
		Mutability [16]byte
		Flush      FlushFunc
		OutPool    *signal.PoolAllocator
		Fn         func(out signal.Floating) (int, error)
	}

	// Processor executes pipe.Processor components.
	Processor struct {
		Mutability [16]byte
		Flush      FlushFunc
		InPool     *signal.PoolAllocator
		OutPool    *signal.PoolAllocator
		Fn         func(in, out signal.Floating) error
	}

	// Sink executes pipe.Sink components.
	Sink struct {
		Mutability [16]byte
		Flush      FlushFunc
		InPool     *signal.PoolAllocator
		Fn         func(in signal.Floating) error
	}
)

// FlushFunc is a closure that triggers pipe component flush function.
type FlushFunc func() error

func (fn FlushFunc) call() error {
	if fn == nil {
		return nil
	}
	return fn()
}

// Run starts the Source runner.
func (r Source) Run(ctx context.Context, mutationsChan chan mutability.Mutations) (<-chan Message, <-chan error) {
	out := make(chan Message, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		// flush on return
		defer func() {
			if err := r.Flush.call(); err != nil {
				errs <- fmt.Errorf("error flushing source: %w", err)
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
				errs <- fmt.Errorf("error mutating source: %w", err)
				return
			}

			outSignal = r.OutPool.GetFloat64()
			if read, err = r.Fn(outSignal); err != nil {
				if err != io.EOF {
					errs <- fmt.Errorf("error running source: %w", err)
				}
				// this buffer wasn't sent, free now
				outSignal.Free(r.OutPool)
				return
			}
			if read != outSignal.Length() {
				outSignal = outSignal.Slice(0, read)
			}

			select {
			case out <- Message{Mutations: mutations, Signal: outSignal}:
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
	go func() {
		defer close(out)
		defer close(errs)
		// flush on return
		defer func() {
			if err := r.Flush.call(); err != nil {
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

			if err = message.Mutations.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating processor: %w", err)
				message.Signal.Free(r.InPool)
				return
			}

			outSignal = r.OutPool.GetFloat64()
			err = r.Fn(message.Signal, outSignal)
			message.Signal.Free(r.InPool)
			if err != nil {
				errs <- fmt.Errorf("error running processor: %w", err)
				// this buffer wasn't sent, free now
				outSignal.Free(r.OutPool)
				return
			}

			select {
			case out <- Message{Mutations: message.Mutations, Signal: outSignal}:
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
	go func() {
		defer close(errs)
		// flush on return
		defer func() {
			if err := r.Flush.call(); err != nil {
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
			if err = message.Mutations.ApplyTo(r.Mutability); err != nil {
				errs <- fmt.Errorf("error mutating sink: %w", err)
				message.Signal.Free(r.InPool) // need to free
				return
			}
			err = r.Fn(message.Signal) // sink a buffer
			message.Signal.Free(r.InPool)
			if err != nil {
				errs <- fmt.Errorf("error running sink: %w", err)
				return
			}
		}
	}()

	return errs
}

// Run starts the runners.
func (r *Runner) Run(ctx context.Context) []<-chan error {
	errChans := make([]<-chan error, 0, 2+len(r.Processors))
	// start source
	out, errs := r.Source.Run(ctx, r.Mutators)
	errChans = append(errChans, errs)

	// start chained processesing
	for _, proc := range r.Processors {
		out, errs = proc.Run(ctx, out)
		errChans = append(errChans, errs)
	}

	errs = r.Sink.Run(ctx, out)
	errChans = append(errChans, errs)
	return errChans
}
