package pipe

import (
	"context"
	"fmt"
	"io"
)

type (
	// Executor executes a single DSP operation.
	Executor interface {
		Execute(context.Context) error
		StartHook(context.Context) error
		FlushHook(context.Context) error
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
	if err := e.StartHook(ctx); err != nil {
		errc <- fmt.Errorf("error starting component: %w", err)
		return
	}
	defer func() {
		if err := e.FlushHook(ctx); err != nil {
			errc <- fmt.Errorf("error flushing component: %w", err)
		}
	}()

	var err error
	for err == nil {
		err = e.Execute(ctx)
	}
	if err != io.EOF {
		errc <- fmt.Errorf("error running component: %w", err)
	}
	return
}
