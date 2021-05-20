package pipe

import (
	"context"
	"fmt"
	"io"

	"pipelined.dev/pipe/mutable"
)

type (
	// executor executes a single pipe operation.
	executor interface {
		// connectOutputs() // resets output fitting states
		execute(context.Context) error
		startHook(context.Context) error
		flushHook(context.Context) error
	}

	// lineExecutor is a sequence of bound and ready-to-run DSP components.
	// All components are running in a single goroutine.
	lineExecutor struct {
		route     int
		started   int
		executors []executor
	}

	// multiLineExecutor allows to run multiple sequences of DSP components
	// in the same goroutine.
	multiLineExecutor struct {
		mutable.Context
		mutable.Destination
		executors []*lineExecutor
	}
)

// Execute all components of the line one-by-one.
func (le *lineExecutor) execute(ctx context.Context) error {
	var err error
	for i := 0; i < le.started; i++ {
		if err = le.executors[i].execute(ctx); err == nil {
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

func (le *lineExecutor) flushHook(ctx context.Context) error {
	var errs execErrors
	for i := 0; i < le.started; i++ {
		if err := le.executors[i].flushHook(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errs.ret()
}

func (le *lineExecutor) startHook(ctx context.Context) error {
	var errs execErrors
	for _, e := range le.executors {
		if err := e.startHook(ctx); err != nil {
			errs = append(errs, err)
			break
		}
		le.started++
	}
	return errs.ret()
}

// startHook calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (mle *multiLineExecutor) startHook(ctx context.Context) error {
	var startErr execErrors
	for i := range mle.executors {
		if err := mle.executors[i].startHook(ctx); err != nil {
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
	flushErr := mle.flushHook(ctx)
	if flushErr != nil {
		err = fmt.Errorf("error flushing lines: %w during start error: %v", flushErr, err)
	}
	return err
}

// Flush flushes all lines.
func (mle *multiLineExecutor) flushHook(ctx context.Context) error {
	var flushErr execErrors
	for _, l := range mle.executors {
		if err := l.flushHook(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (mle *multiLineExecutor) execute(ctx context.Context) error {
	var err error
	for i := 0; i < len(mle.executors); {
		if err = mle.executors[i].execute(ctx); err == nil {
			i++
			continue
		}
		if err == io.EOF {
			if flushErr := mle.executors[i].flushHook(ctx); flushErr != nil {
				return flushErr
			}
			mle.executors = append(mle.executors[:i], mle.executors[i+1:]...)
			if len(mle.executors) > 0 {
				continue
			}
		}
		return err
	}
	return nil
}

func (mle *multiLineExecutor) addRoute(ctx context.Context, r *route, routeIdx int, cancelFn context.CancelFunc) mutable.Mutation {
	return mle.Context.Mutate(func() error {
		le := r.executor(mle.Destination, routeIdx)
		if err := le.startHook(ctx); err != nil {
			return fmt.Errorf("line failed to start: %w", err)
		}
		mle.executors = append(mle.executors, le)
		cancelFn()
		return nil
	})
}

func (mle *multiLineExecutor) insertProcessor(ctx context.Context, line, pos int, proc Processor, cancelFn context.CancelFunc) mutable.Mutation {
	return mle.Context.Mutate(func() error {
		defer cancelFn()
		for _, e := range mle.executors {
			if e.route != line {
				continue
			}

			inserter := e.executors[pos].(interface{ insert(out) })
			inserter.insert(proc.out)

			err := proc.startHook(ctx)
			if err != nil {
				return fmt.Errorf("error starting processor: %w", err)
			}
			e.executors = append(e.executors, nil)
			copy(e.executors[pos+1:], e.executors[pos:])
			e.executors[pos] = &proc
			e.started++
			break
		}
		return nil
	})
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
