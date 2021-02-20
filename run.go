package pipe

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/pipe/mutable"
)

type (
	// executor executes a single DSP operation.
	executor interface {
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
		executors []executor
	}

	// MultiLineRunner allows to run multiple sequences of DSP components
	// in the same goroutine.
	MultiLineRunner struct {
		Lines []*LineRunner
	}
)

func (r *LineRunner) start(ctx context.Context, merger *errorMerger) {
	r.bind()
	for _, e := range r.executors {
		merger.add(start(ctx, e))
	}
}

func (r *MultiLineRunner) start(ctx context.Context, merger *errorMerger) {
	r.bind()
	merger.add(start(ctx, r))
}

func (r *LineRunner) bindContexts(p mutable.Pusher, d mutable.Destination) {
	p.AddDestination(r.executors[0].(Source).Context, d)
	// sync line shares context for all components
	if !r.context.IsMutable() {
		return
	}

	for i := 1; i < len(r.executors)-1; i++ {
		p.AddDestination(r.executors[i].(Processor).Context, d)
	}
	p.AddDestination(r.executors[len(r.executors)-1].(Sink).Context, d)
}

func (r *LineRunner) bind() {
	e := r.executors[0].(Source)
	e.out.Sender.Bind()

	for i := 1; i < len(r.executors)-1; i++ {
		e := r.executors[i].(Processor)
		e.out.Sender.Bind()
	}
}

func (r *MultiLineRunner) bind() {
	for i := range r.Lines {
		r.Lines[i].bind()
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

// Run executes line synchonously in a single goroutine.
func (r *LineRunner) Run(ctx context.Context) error {
	r.bind()
	return run(ctx, r)
}

// Run executes multiple lines synchonously in a single
// goroutine.
func (r *MultiLineRunner) Run(ctx context.Context) error {
	r.bind()
	return run(ctx, r)
}

// startHook calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (r *MultiLineRunner) startHook(ctx context.Context) error {
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
func (r *MultiLineRunner) flushHook(ctx context.Context) error {
	var flushErr execErrors
	for _, l := range r.Lines {
		if err := l.flushHook(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (r *MultiLineRunner) execute(ctx context.Context) error {
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

// start executes dsp component in an async context. For successfully
// started executor an error is returned immediately after it occured.
func start(ctx context.Context, e executor) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		if err := e.startHook(ctx); err != nil {
			errc <- fmt.Errorf("error starting: %w", err)
			return
		}
		defer func() {
			if err := e.flushHook(ctx); err != nil {
				errc <- fmt.Errorf("error flushing: %w", err)
			}
		}()

		var err error
		for err == nil {
			err = e.execute(ctx)
		}
		if err != io.EOF {
			errc <- fmt.Errorf("error running: %w", err)
		}
		return
	}()
	return errc
}

// run executes dsp component in a sync context. For successfully started
// components no errors returned until flush is performed.
func run(ctx context.Context, e executor) (errExec error) {
	if errStart := e.startHook(ctx); errStart != nil {
		return fmt.Errorf("error starting: %w", errStart)
	}
	defer func() {
		errFlush := e.flushHook(ctx)
		if errFlush == nil && errExec == nil {
			return
		}
		errExec = &ErrorRun{
			ErrFlush: fmt.Errorf("error flushing: %w", errFlush),
			ErrExec:  errExec,
		}
	}()

	// var err error
	for errExec == nil {
		errExec = e.execute(ctx)
	}
	if errExec == io.EOF {
		errExec = nil
	} else {
		errExec = fmt.Errorf("error running: %w", errExec)
	}
	return
}
