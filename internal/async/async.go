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
	// Executor executes a single DSP operation.
	Executor interface {
		Execute(context.Context) error
		Start(context.Context) error
		Flush(context.Context) error
	}

	// ComponentStarter executes components.
	ComponentStarter struct {
		Executor
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

// Starter returns async runner for pipe.source components.
func Starter(e Executor) *ComponentStarter {
	return &ComponentStarter{
		Executor: e,
	}
}

// Start starts the component runner.
func (r *ComponentStarter) Start(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go r.run(ctx, errc)
	return errc
}

func (r *ComponentStarter) run(ctx context.Context, errc chan<- error) {
	defer close(errc)
	if err := r.Executor.Start(ctx); err != nil {
		errc <- fmt.Errorf("error starting component: %w", err)
		return
	}
	defer func() {
		if err := r.Flush(ctx); err != nil {
			errc <- fmt.Errorf("error flushing component: %w", err)
		}
	}()

	var err error
	for err == nil {
		err = r.Executor.Execute(ctx)
	}
	if err != io.EOF {
		errc <- fmt.Errorf("error running component: %w", err)
	}
	return
}
