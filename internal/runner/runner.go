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
	Runner interface {
		Run(context.Context) <-chan error
		Out() <-chan Message
		Insert(Runner)
	}

	// Line defines the sequence of executors.
	// Line struct {
	// 	Source     source
	// 	Processors []Processor
	// 	Sink
	// }

	// source executes pipe.source components.
	source struct {
		mutations  chan mutability.Mutations
		mutability mutability.Mutability
		Start      HookFunc
		Flush      HookFunc
		OutPool    *signal.PoolAllocator
		Fn         SourceFunc
		out
	}

	// processor executes pipe.processor components.
	processor struct {
		mutability mutability.Mutability
		Start      HookFunc
		Flush      HookFunc
		InPool     *signal.PoolAllocator
		OutPool    *signal.PoolAllocator
		In         <-chan Message
		Fn         ProcessFunc
		out
	}

	// sink executes pipe.sink components.
	sink struct {
		mutability mutability.Mutability
		Start      HookFunc
		Flush      HookFunc
		InPool     *signal.PoolAllocator
		Fn         SinkFunc
		In         <-chan Message
	}
)

type (
	SourceFunc  func(out signal.Floating) (int, error)
	ProcessFunc func(in, out signal.Floating) error
	SinkFunc    func(in signal.Floating) error
)

type out chan Message

func (o out) Out() <-chan Message {
	return o
}

// HookFunc is a closure that triggers pipe component hook function.
type HookFunc func(ctx context.Context) error

func (fn HookFunc) call(ctx context.Context) error {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

func Source(mc chan mutability.Mutations, m mutability.Mutability, p *signal.PoolAllocator, fn SourceFunc, start, flush HookFunc) Runner {
	return &source{
		mutations:  mc,
		mutability: m,
		OutPool:    p,
		Fn:         fn,
		Start:      start,
		Flush:      flush,
		out:        make(chan Message, 1),
	}
}

func Processor(m mutability.Mutability, in <-chan Message, inp, outp *signal.PoolAllocator, fn ProcessFunc, start, flush HookFunc) Runner {
	return &processor{
		mutability: m,
		In:         in,
		InPool:     inp,
		OutPool:    outp,
		Fn:         fn,
		Start:      start,
		Flush:      flush,
		out:        make(chan Message, 1),
	}
}

func Sink(m mutability.Mutability, in <-chan Message, p *signal.PoolAllocator, fn SinkFunc, start, flush HookFunc) Runner {
	return &sink{
		mutability: m,
		In:         in,
		InPool:     p,
		Fn:         fn,
		Start:      start,
		Flush:      flush,
	}
}

// Run starts the Source runner.
func (r *source) Run(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(r.out)
		defer close(errc)
		if err := r.Start.call(ctx); err != nil {
			errc <- fmt.Errorf("error starting source: %w", err)
		}
		// flush shouldn't be executed if start has failed.
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errc <- fmt.Errorf("error flushing source: %w", err)
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
			case mutations = <-r.mutations:
			case <-ctx.Done():
				return
			default:
			}

			if err = mutations.ApplyTo(r.mutability); err != nil {
				errc <- fmt.Errorf("error mutating source: %w", err)
				return
			}

			outSignal = r.OutPool.GetFloat64()
			if read, err = r.Fn(outSignal); err != nil {
				if err != io.EOF {
					errc <- fmt.Errorf("error running source: %w", err)
				}
				// this buffer wasn't sent, free now
				outSignal.Free(r.OutPool)
				return
			}
			if read != outSignal.Length() {
				outSignal = outSignal.Slice(0, read)
			}

			select {
			case r.out <- Message{Mutations: mutations, Signal: outSignal}:
				mutations = nil
			case <-ctx.Done():
				return
			}
		}
	}()
	return errc
}

func (r *source) Insert(Runner) {
	panic("source cannot insert")
}

// Run starts the Processor runner.
func (r *processor) Run(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(r.out)
		defer close(errc)
		if err := r.Start.call(ctx); err != nil {
			errc <- fmt.Errorf("error starting source: %w", err)
		}
		// flush shouldn't be executed if start has failed.
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errc <- fmt.Errorf("error flushing processor: %w", err)
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
			case message, ok = <-r.In:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			if err = message.Mutations.ApplyTo(r.mutability); err != nil {
				errc <- fmt.Errorf("error mutating processor: %w", err)
				message.Signal.Free(r.InPool)
				return
			}

			outSignal = r.OutPool.GetFloat64()
			err = r.Fn(message.Signal, outSignal)
			message.Signal.Free(r.InPool)
			if err != nil {
				errc <- fmt.Errorf("error running processor: %w", err)
				// this buffer wasn't sent, free now
				outSignal.Free(r.OutPool)
				return
			}

			select {
			case r.out <- Message{Mutations: message.Mutations, Signal: outSignal}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return errc
}

func (r *processor) Insert(nr Runner) {
	// var in chan Message
	// if len(r.Processors) == 0 {
	// 	in = r.Source.Out
	// }

	// if pos == 0 {

	// }
	panic("not implemented")
}

// Run starts the sink runner.
func (r *sink) Run(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		if err := r.Start.call(ctx); err != nil {
			errc <- fmt.Errorf("error starting source: %w", err)
		}
		// flush shouldn't be executed if start has failed.
		defer func() {
			if err := r.Flush.call(ctx); err != nil {
				errc <- fmt.Errorf("error flushing sink: %w", err)
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
			case message, ok = <-r.In:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			// apply Mutators
			if err = message.Mutations.ApplyTo(r.mutability); err != nil {
				errc <- fmt.Errorf("error mutating sink: %w", err)
				message.Signal.Free(r.InPool) // need to free
				return
			}
			err = r.Fn(message.Signal) // sink a buffer
			message.Signal.Free(r.InPool)
			if err != nil {
				errc <- fmt.Errorf("error running sink: %w", err)
				return
			}
		}
	}()

	return errc
}

func (r *sink) Insert(nr Runner) {
	panic("not implemented")
}

func (*sink) Out() <-chan Message {
	return nil
}
