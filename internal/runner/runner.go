package runner

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"pipelined.dev/signal"

	"pipelined.dev/pipe/metric"
	"pipelined.dev/pipe/mutator"
)

// Pool provides pooling of signal.Float64 buffers.
type Pool interface {
	Alloc() signal.Float64
	Free(signal.Float64)
}

// Message is a main structure for pipe transport
type Message struct {
	SinkRefs int32            // number of sinks referencing buffer in this message.
	Buffer   signal.Float64   // Buffer of message.
	Mutators mutator.Mutators // Mutators for pipe.
}

type (
	// Pump executes pipe.Pump components.
	Pump struct {
		mutator.Receiver
		signal.SampleRate
		NumChannels int
		Fn          func(out signal.Float64) error
		Flush       func(context.Context) error
		Meter       metric.ResetFunc
		outputPool  Pool
	}

	// Processor executes pipe.Processor components.
	Processor struct {
		mutator.Receiver
		signal.SampleRate
		NumChannels int
		Fn          func(in, out signal.Float64) error
		Flush       func(context.Context) error
		Meter       metric.ResetFunc
		inputPool   Pool
		outputPool  Pool
	}

	// Sink executes pipe.Sink components.
	Sink struct {
		mutator.Receiver
		signal.SampleRate
		NumChannels int
		Fn          func(in signal.Float64) error
		Flush       func(context.Context) error
		Meter       metric.ResetFunc
		inputPool   Pool
	}
)

// Run starts the Pump runner.
func (r Pump) Run(ctx context.Context, p Pool, give chan<- chan mutator.Mutators, take chan mutator.Mutators) (<-chan Message, <-chan error) {
	out := make(chan Message, 1)
	errs := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(out)
		defer close(errs)
		// Flush hook on return
		defer func() {
			if err := r.Flush(ctx); err != nil {
				errs <- fmt.Errorf("error flushing pump: %w", err)
			}
		}()
		var err error
		var Mutators mutator.Mutators
		for {
			// request new message
			select {
			case give <- take:
			case <-ctx.Done():
				return
			}

			// receive new message
			select {
			case Mutators = <-take:
			case <-ctx.Done():
				return
			}
			m := Message{Mutators: Mutators}
			m.Mutators.ApplyTo(&r.Receiver) // apply Mutators

			// allocate new buffer
			m.Buffer = p.Alloc()

			err = r.Fn(m.Buffer)   // pump new buffer
			meter(m.Buffer.Size()) // capture metrics
			// handle error
			if err != nil {
				if err != io.EOF {
					errs <- fmt.Errorf("error running pump: %w", err)
				}
				return
			}

			// push message further
			select {
			case out <- m:
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
			if err := r.Flush(ctx); err != nil {
				errs <- fmt.Errorf("error flushing processor: %w", err)
			}
		}()
		var err error
		var m Message
		var ok bool
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

			m.Mutators.ApplyTo(&r.Receiver) // apply Mutators
			// err = r.Fn(m.Buffer)        // process new buffer
			if err != nil {
				errs <- fmt.Errorf("error running processor: %w", err)
				return
			}

			meter(m.Buffer.Size()) // capture metrics

			// send message further
			select {
			case out <- m:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errs
}

// Run starts the sink runner.
func (r Sink) Run(ctx context.Context, p Pool, in <-chan Message) <-chan error {
	errs := make(chan error, 1)
	meter := r.Meter()
	go func() {
		defer close(errs)
		// Flush hook on return
		defer func() {
			if err := r.Flush(ctx); err != nil {
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

			m.Mutators.ApplyTo(&r.Receiver) // apply Mutators
			err := r.Fn(m.Buffer)           // sink a buffer
			if err != nil {
				errs <- fmt.Errorf("error running sink: %w", err)
				return
			}
			meter(m.Buffer.Size()) // capture metrics
			if atomic.AddInt32(&m.SinkRefs, -1) == 0 {
				p.Free(m.Buffer)
			}
		}
	}()

	return errs
}

// Broadcast passes messages to all sinks.
func Broadcast(ctx context.Context, p Pool, sinks []Sink, in <-chan Message) []<-chan error {
	//init errs for sinks error channels
	errs := make([]<-chan error, 0, len(sinks))
	//list of channels for broadcast
	broadcasts := make([]chan Message, len(sinks))
	for i := range broadcasts {
		broadcasts[i] = make(chan Message, 1)
	}

	//start broadcast
	for i, s := range sinks {
		errs = append(errs, s.Run(ctx, p, broadcasts[i]))
	}

	go func() {
		//close broadcasts on return
		defer func() {
			for i := range broadcasts {
				close(broadcasts[i])
			}
		}()
		for msg := range in {
			for i := range broadcasts {
				m := Message{
					SinkRefs: int32(len(broadcasts)),
					Buffer:   msg.Buffer,
					Mutators: msg.Mutators.Detach(&sinks[i].Receiver),
				}
				select {
				case broadcasts[i] <- m:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return errs
}
