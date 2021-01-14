package execution

import (
	"context"
	"fmt"
	"io"
)

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
