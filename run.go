package pipe

import (
	"context"
	"fmt"
	"io"
)

type (
	// Executor executes a single DSP operation.
	Executor interface {
		execute(context.Context) error
		startHook(context.Context) error
		flushHook(context.Context) error
	}
)

// Run the component runner.
func Run(ctx context.Context, e Executor) <-chan error {
	errc := make(chan error, 1)
	go runExecutor(ctx, e, errc)
	return errc
}

func runExecutor(ctx context.Context, e Executor, errc chan<- error) {
	defer close(errc)
	if err := e.startHook(ctx); err != nil {
		errc <- fmt.Errorf("error starting component: %w", err)
		return
	}
	defer func() {
		if err := e.flushHook(ctx); err != nil {
			errc <- fmt.Errorf("error flushing component: %w", err)
		}
	}()

	var err error
	for err == nil {
		err = e.execute(ctx)
	}
	if err != io.EOF {
		errc <- fmt.Errorf("error running component: %w", err)
	}
	return
}
