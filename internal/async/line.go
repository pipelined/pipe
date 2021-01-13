package async

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/pipe/mutable"
)

type LineStarter struct {
	Mutations chan mutable.Mutations
	Executors []Executor
}

func (r *LineStarter) Start(ctx context.Context) <-chan error {
	// todo: determine buffer size
	errc := make(chan error, 1)
	go r.run(ctx, errc)
	return errc
}

// TODO: handle context
// TODO: handle mutations
// TODO: handle errors
func (r *LineStarter) run(ctx context.Context, errc chan error) {
	defer close(errc)

	// start fn
	var started int
	for i := range r.Executors {
		if err := r.Executors[i].Start(ctx); err != nil {
			errc <- fmt.Errorf("error starting %d component: %w", i, err)
		}
		started++
	}

	defer r.flush(ctx, errc)
	// check if all lines started and return if not
	if started != len(r.Executors) {
		return
	}

	for {
		for i := range r.Executors {
			if err := r.Executors[i].Execute(ctx); err != nil {
				if err != io.EOF {
					errc <- fmt.Errorf("error running component: %w", err)
				}
				return
			}
		}
	}
}

func (r *LineStarter) flush(ctx context.Context, errc chan error) {
	for _, e := range r.Executors {
		if err := e.Flush(ctx); err != nil {
			errc <- fmt.Errorf("error flushing component: %w", err)
		}
	}
}

// func (lr Line) flush(ctx context.Context, errc chan error) {
// 	err := callHook(ctx, lr.Source.FlushFunc)
// 	if err != nil {
// 		errc <- err
// 	}

// 	for i := range lr.Processors {
// 		err = callHook(ctx, lr.Processors[i].FlushFunc)
// 		if err != nil {
// 			errc <- err
// 		}
// 	}
// 	err = callHook(ctx, lr.Sink.FlushFunc)
// 	if err != nil {
// 		errc <- err
// 	}
// }

// func (lr Lines) Out() <-chan Message {
// 	return nil
// }

// func (lr Lines) OutputPool() *signal.PoolAllocator {
// 	return nil
// }

// func (lr Lines) Insert(Starter, mutable.MutatorFunc) mutable.Mutation {
// 	return mutable.Mutation{}
// }
