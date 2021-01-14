package runtime

import (
	"context"
	"fmt"

	"pipelined.dev/pipe/mutable"
)

type Lines struct {
	Mutations chan mutable.Mutations
	Lines     []Line
}

// func (r *LinesExecutor) Starter(ctx context.Context) <-chan error {
// 	errc := make(chan error, 1)
// 	go r.run(ctx, errc)
// 	return errc
// }

func (r *Lines) Start(ctx context.Context) error {
	var startErr lineErrors
	for i := range r.Lines {
		if err := r.Lines[i].Start(ctx); err != nil {
			startErr = append(startErr, err)
			break
		}
	}

	// all started smooth
	if len(startErr) == 0 {
		return nil
	}
	// wrap start error
	err := fmt.Errorf("error starting lines: %w", startErr.ret())
	// need to flush sucessfully started components
	flushErr := r.Flush(ctx)
	if flushErr != nil {
		err = fmt.Errorf("error flushing lines during start error: %w", flushErr)
	}
	return err
}

func (r *Lines) Flush(ctx context.Context) error {
	var flushErr lineErrors
	for i := range r.Lines {
		if err := r.Lines[i].Flush(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

func (r *Lines) Execute(ctx context.Context) error {
	for i := range r.Lines {
		if err := r.Lines[i].Execute(ctx); err != nil {
			// TODO: handle EOF
			return err
		}
	}
	return nil
}

// func (lr Lines) Out() <-chan Message {
// 	return nil
// }

// func (lr Lines) OutputPool() *signal.PoolAllocator {
// 	return nil
// }

// func (lr Lines) Insert(Starter, mutable.MutatorFunc) mutable.Mutation {
// 	return mutable.Mutation{}
// }
