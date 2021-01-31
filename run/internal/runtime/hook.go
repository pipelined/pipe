package runtime

import (
	"context"

	"pipelined.dev/pipe"
)

type (
	// StartFunc is a closure that triggers pipe component start hook.
	StartFunc pipe.StartFunc
	// FlushFunc is a closure that triggers pipe component start hook.
	FlushFunc pipe.FlushFunc
)

// Start calls the start hook.
func (fn StartFunc) Start(ctx context.Context) error {
	return callHook(ctx, fn)
}

// Flush calls the flush hook.
func (fn FlushFunc) Flush(ctx context.Context) error {
	return callHook(ctx, fn)
}

func callHook(ctx context.Context, hook func(context.Context) error) error {
	if hook == nil {
		return nil
	}
	return hook(ctx)
}
