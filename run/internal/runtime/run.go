package runtime

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
)

// Run the component runner.
func Run(ctx context.Context, e Executor) <-chan error {
	errc := make(chan error, 1)
	go run(ctx, e, errc)
	return errc
}

func run(ctx context.Context, e Executor, errc chan<- error) {
	defer close(errc)
	if err := e.Start(ctx); err != nil {
		errc <- fmt.Errorf("error starting component: %w", err)
		return
	}
	defer func() {
		if err := e.Flush(ctx); err != nil {
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
