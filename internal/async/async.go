package async

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/mutable"
)

// Message is a main structure for pipe transport
type Message struct {
	Signal            signal.Floating // Buffer of message.
	mutable.Mutations                 // Mutators for pipe.
}

type (
	// Runner is asynchronous component executor.
	Runner interface {
		Run(context.Context) <-chan error
		Out() <-chan Message
		OutputPool() *signal.PoolAllocator
		Insert(Runner, mutable.MutatorFunc) mutable.Mutation
	}

	// source executes pipe.source components.
	source struct {
		mutations  chan mutable.Mutations
		mctx       mutable.Context
		outputPool *signal.PoolAllocator
		startFn    HookFunc
		flushFn    HookFunc
		sourceFn   SourceFunc
		out
	}

	// processor executes pipe.processor components.
	processor struct {
		mctx       mutable.Context
		inputPool  *signal.PoolAllocator
		outputPool *signal.PoolAllocator
		startFn    HookFunc
		flushFn    HookFunc
		processFn  ProcessFunc
		in         <-chan Message
		out
	}

	// sink executes pipe.sink components.
	sink struct {
		mctx      mutable.Context
		startFn   HookFunc
		flushFn   HookFunc
		inputPool *signal.PoolAllocator
		sinkFn    SinkFunc
		in        <-chan Message
	}
)

type (
	// SourceFunc is a wrapper type of source closure.
	SourceFunc func(out signal.Floating) (int, error)
	// ProcessFunc is a wrapper type of processor closure.
	ProcessFunc func(in, out signal.Floating) error
	// SinkFunc is a wrapper type of sink closure.
	SinkFunc func(in signal.Floating) error
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

// Source returns async runner for pipe.source components.
func Source(mc chan mutable.Mutations, m mutable.Context, p *signal.PoolAllocator, fn SourceFunc, start, flush HookFunc) Runner {
	return &source{
		mutations:  mc,
		mctx:       m,
		outputPool: p,
		sourceFn:   fn,
		startFn:    start,
		flushFn:    flush,
		out:        make(chan Message, 1),
	}
}

// Processor returns async runner for pipe.processor components.
func Processor(m mutable.Context, in <-chan Message, inp, outp *signal.PoolAllocator, fn ProcessFunc, start, flush HookFunc) Runner {
	return &processor{
		mctx:       m,
		in:         in,
		inputPool:  inp,
		outputPool: outp,
		processFn:  fn,
		startFn:    start,
		flushFn:    flush,
		out:        make(chan Message, 1),
	}
}

// Sink returns async runner for pipe.sink components.
func Sink(m mutable.Context, in <-chan Message, p *signal.PoolAllocator, fn SinkFunc, start, flush HookFunc) Runner {
	return &sink{
		mctx:      m,
		in:        in,
		inputPool: p,
		sinkFn:    fn,
		startFn:   start,
		flushFn:   flush,
	}
}

// Run starts the Source runner.
func (r *source) Run(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(r.out)
		defer close(errc)
		if err := r.startFn.call(ctx); err != nil {
			errc <- fmt.Errorf("error starting source: %w", err)
		}
		// flush shouldn't be executed if start has failed.
		defer func() {
			if err := r.flushFn.call(ctx); err != nil {
				errc <- fmt.Errorf("error flushing source: %w", err)
			}
		}()
		var (
			read      int
			mutations mutable.Mutations
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

			if err = mutations.ApplyTo(r.mctx); err != nil {
				errc <- fmt.Errorf("error mutating source: %w", err)
				return
			}

			outSignal = r.outputPool.GetFloat64()
			if read, err = r.sourceFn(outSignal); err != nil {
				if err != io.EOF {
					errc <- fmt.Errorf("error running source: %w", err)
				}
				// this buffer wasn't sent, free now
				outSignal.Free(r.outputPool)
				return
			}
			if read != outSignal.Length() {
				fmt.Printf("read: %v\n", read)
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

func (r *source) Insert(Runner, mutable.MutatorFunc) mutable.Mutation {
	panic("source cannot insert")
}

func (r *source) OutputPool() *signal.PoolAllocator {
	return r.outputPool
}

// Run starts the Processor runner.
func (r *processor) Run(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(r.out)
		defer close(errc)
		if err := r.startFn.call(ctx); err != nil {
			errc <- fmt.Errorf("error starting source: %w", err)
		}
		// flush shouldn't be executed if start has failed.
		defer func() {
			if err := r.flushFn.call(ctx); err != nil {
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
			case message, ok = <-r.in:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			if err = message.Mutations.ApplyTo(r.mctx); err != nil {
				errc <- fmt.Errorf("error mutating processor: %w", err)
				message.Signal.Free(r.inputPool)
				return
			}

			outSignal = r.outputPool.GetFloat64()
			err = r.processFn(message.Signal, outSignal)
			message.Signal.Free(r.inputPool)
			if err != nil {
				errc <- fmt.Errorf("error running processor: %w", err)
				// this buffer wasn't sent, free now
				outSignal.Free(r.outputPool)
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

func (r *processor) Insert(newRunner Runner, insertHook mutable.MutatorFunc) mutable.Mutation {
	return r.mctx.Mutate(func() error {
		r.in = newRunner.Out()
		return insertHook()
	})
}

func (r *processor) OutputPool() *signal.PoolAllocator {
	return r.outputPool
}

// Run starts the sink runner.
func (r *sink) Run(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		if err := r.startFn.call(ctx); err != nil {
			errc <- fmt.Errorf("error starting source: %w", err)
		}
		// flush shouldn't be executed if start has failed.
		defer func() {
			if err := r.flushFn.call(ctx); err != nil {
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
			case message, ok = <-r.in:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}

			// apply Mutators
			if err = message.Mutations.ApplyTo(r.mctx); err != nil {
				errc <- fmt.Errorf("error mutating sink: %w", err)
				message.Signal.Free(r.inputPool) // need to free
				return
			}
			err = r.sinkFn(message.Signal) // sink a buffer
			message.Signal.Free(r.inputPool)
			if err != nil {
				errc <- fmt.Errorf("error running sink: %w", err)
				return
			}
		}
	}()

	return errc
}

func (r *sink) Insert(newRunner Runner, insertHook mutable.MutatorFunc) mutable.Mutation {
	return r.mctx.Mutate(func() error {
		r.in = newRunner.Out()
		return insertHook()
	})
}

func (r *sink) OutputPool() *signal.PoolAllocator {
	return nil
}

func (*sink) Out() <-chan Message {
	return nil
}
