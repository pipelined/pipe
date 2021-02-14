package pipe

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/pipe/mutable"
)

type (
	// Executor executes a single DSP operation.
	Executor interface {
		execute(context.Context) error
		startHook(context.Context) error
		flushHook(context.Context) error
	}

	// LineRunner is a sequence of bound and ready-to-run DSP components.
	// It supports two execution modes: every component is running in its
	// own goroutine (default) and running all components in a single
	// goroutine.
	LineRunner struct {
		context   mutable.Context
		started   int
		executors []Executor
	}

	// MultilineRunner allows to run multiple sequences of DSP components
	// in the same goroutine.
	MultilineRunner struct {
		mutable.Context
		Lines []*LineRunner
	}
)

func (r *LineRunner) run(ctx context.Context, merger *errorMerger) {
	r.Bind()
	for _, e := range r.executors {
		merger.add(Run(ctx, e))
	}
}

func (r *MultilineRunner) run(ctx context.Context, merger *errorMerger) {
	r.Bind()
	merger.add(Run(ctx, r))
}

func (r *LineRunner) bindContexts(contexts map[mutable.Context]chan mutable.Mutations, mc chan mutable.Mutations) {
	contexts[r.executors[0].(Source).Context] = mc
	if !r.context.IsMutable() {
		return
	}

	for i := 1; i < len(r.executors)-1; i++ {
		contexts[r.executors[i].(Processor).Context] = mc
	}
	contexts[r.executors[len(r.executors)-1].(Sink).Context] = mc
}

func (r *LineRunner) Bind() {
	e := r.executors[0].(Source)
	e.out.Sender.Bind()

	for i := 1; i < len(r.executors)-1; i++ {
		e := r.executors[i].(Processor)
		e.out.Sender.Bind()
	}
}

func (r *MultilineRunner) Bind() {
	for i := range r.Lines {
		r.Lines[i].Bind()
	}
}

// Execute all components of the line one-by-one.
func (r *LineRunner) execute(ctx context.Context) error {
	var err error
	for _, e := range r.executors {
		if err = e.execute(ctx); err == nil {
			continue
		}
		if err == io.EOF {
			// continue execution to propagate EOF
			continue
		}
		return err
	}
	// if no other errors were met, EOF will be returned
	return err
}

func (r *LineRunner) flushHook(ctx context.Context) error {
	var errs execErrors
	for i := 0; i < r.started; i++ {
		if err := r.executors[i].flushHook(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errs.ret()
}

func (r *LineRunner) startHook(ctx context.Context) error {
	var errs execErrors
	for _, e := range r.executors {
		if err := e.startHook(ctx); err != nil {
			errs = append(errs, err)
			break
		}
		r.started++
	}
	return errs.ret()
}

// startHook calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (r *MultilineRunner) startHook(ctx context.Context) error {
	var startErr execErrors
	for i := range r.Lines {
		if err := r.Lines[i].startHook(ctx); err != nil {
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
	flushErr := r.flushHook(ctx)
	if flushErr != nil {
		err = fmt.Errorf("error flushing lines: %w during start error: %v", flushErr, err)
	}
	return err
}

// Flush flushes all lines.
func (r *MultilineRunner) flushHook(ctx context.Context) error {
	var flushErr execErrors
	for _, l := range r.Lines {
		if err := l.flushHook(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (r *MultilineRunner) execute(ctx context.Context) error {
	var err error
	for i := 0; i < len(r.Lines); {
		if err = r.Lines[i].execute(ctx); err == nil {
			i++
			continue
		}
		if err == io.EOF {
			if flushErr := r.Lines[i].flushHook(ctx); flushErr != nil {
				return flushErr
			}
			r.Lines = append(r.Lines[:i], r.Lines[i+1:]...)
			if len(r.Lines) > 0 {
				continue
			}
		}
		return err
	}
	return nil
}

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
