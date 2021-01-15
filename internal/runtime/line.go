package runtime

import (
	"context"
	"fmt"
	"strings"

	"pipelined.dev/pipe/mutable"
)

type Line struct {
	started   int
	Executors []Executor
}

func (l *Line) Execute(ctx context.Context) error {
	for _, e := range l.Executors {
		if err := e.Execute(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (l *Line) Flush(ctx context.Context) error {
	var errs lineErrors
	for i := 0; i < l.started; i++ {
		if err := l.Executors[i].Flush(ctx); err != nil {
			errs = append(errs, err)
			break
		}
	}
	return errs.ret()
}

func (l *Line) Start(ctx context.Context) error {
	var errs lineErrors
	for _, e := range l.Executors {
		if err := e.Start(ctx); err != nil {
			errs = append(errs, err)
			break
		}
		l.started++
	}
	return errs.ret()
}

type lineErrors []error

func (e lineErrors) Error() string {
	s := []string{}
	for _, se := range e {
		s = append(s, se.Error())
	}
	return strings.Join(s, ",")
}

// ret returns untyped nil if error is list is empty.
func (e lineErrors) ret() error {
	if len(e) > 0 {
		return e
	}
	return nil
}

type Lines struct {
	Mutations chan mutable.Mutations
	Lines     []Line
}

func (e *Lines) Start(ctx context.Context) error {
	var startErr lineErrors
	for i := range e.Lines {
		if err := e.Lines[i].Start(ctx); err != nil {
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
	flushErr := e.Flush(ctx)
	if flushErr != nil {
		err = fmt.Errorf("error flushing lines during start error: %w", flushErr)
	}
	return err
}

func (e *Lines) Flush(ctx context.Context) error {
	var flushErr lineErrors
	for i := range e.Lines {
		if err := e.Lines[i].Flush(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

func (e *Lines) Execute(ctx context.Context) error {
	for i := range e.Lines {
		if err := e.Lines[i].Execute(ctx); err != nil {
			// TODO: handle EOF
			// TODO: handle ErrContextDone
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
