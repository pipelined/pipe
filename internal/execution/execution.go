package execution

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

// TBD
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

// TBD
