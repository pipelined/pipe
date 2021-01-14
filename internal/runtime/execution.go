package runtime

import (
	"context"
	"errors"

	"pipelined.dev/signal"
)

var ErrContextDone = errors.New("context is done")

type (
	// SourceFunc is a wrapper type of source closure.
	SourceFunc func(out signal.Floating) (int, error)
	// ProcessFunc is a wrapper type of processor closure.
	ProcessFunc func(in, out signal.Floating) error
	// SinkFunc is a wrapper type of sink closure.
	SinkFunc func(in signal.Floating) error
)

type (
	// StartFunc is a closure that triggers pipe component start hook.
	StartFunc func(ctx context.Context) error
	// FlushFunc is a closure that triggers pipe component start hook.
	FlushFunc func(ctx context.Context) error
)

// Start calls the start hook.
func (fn StartFunc) Start(ctx context.Context) error {
	return callHook(ctx, fn)
}

// Flush calls the flush hook.
func (fn StartFunc) Flush(ctx context.Context) error {
	return callHook(ctx, fn)
}

func callHook(ctx context.Context, hook func(context.Context) error) error {
	if hook == nil {
		return nil
	}
	return hook(ctx)
}
