package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

type (
	// Lines executes multiple lines in the same goroutine.
	Lines struct {
		Mutations chan mutable.Mutations
		Lines     []Line
	}

	// Line represents a sequence of components executors.
	Line struct {
		started   int
		Executors []Executor
	}

	lineErrors []error
)

// Start calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (e *Lines) Start(ctx context.Context) error {
	var startErr lineErrors
	for i := range e.Lines {
		if err := e.Lines[i].start(ctx); err != nil {
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

// Flush flushes all lines.
func (e *Lines) Flush(ctx context.Context) error {
	var flushErr lineErrors
	for i := range e.Lines {
		if err := e.Lines[i].flush(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (e *Lines) Execute(ctx context.Context) error {
	var err error
	for i := 0; i < len(e.Lines); {
		if err = e.Lines[i].execute(ctx); err == nil {
			i++
			continue
		}
		if err == io.EOF {
			if flushErr := e.Lines[i].flush(ctx); flushErr != nil {
				return flushErr
			}
			e.Lines = append(e.Lines[:i], e.Lines[i+1:]...)
			if len(e.Lines) > 0 {
				continue
			}
		}
		return err
	}
	return nil
}

// Execute all components of the line one-by-one.
func (l *Line) execute(ctx context.Context) error {
	var err error
	for _, e := range l.Executors {
		if err = e.Execute(ctx); err == nil {
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

func (l *Line) flush(ctx context.Context) error {
	var errs lineErrors
	for i := 0; i < l.started; i++ {
		if err := l.Executors[i].Flush(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errs.ret()
}

func (l *Line) start(ctx context.Context) error {
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

// LineExecutor returns executor for components bound into line.
func LineExecutor(l *pipe.Line, mc chan mutable.Mutations) Line {
	line := Line{
		Executors: make([]Executor, 0, 2+len(l.Processors)),
	}
	var (
		sender, receiver Link
		input, output    *signal.PoolAllocator
	)
	sender = SyncLink()
	output = l.SourceOutputPool()
	line.Executors = append(line.Executors,
		SourceExecutor(
			l.Source,
			mc,
			output,
			sender,
		),
	)

	for i := range l.Processors {
		receiver, sender = sender, AsyncLink()
		input, output = output, l.ProcessorOutputPool(i)
		line.Executors = append(line.Executors,
			ProcessExecutor(
				l.Processors[i],
				input,
				output,
				receiver,
				sender,
			),
		)
	}
	line.Executors = append(line.Executors, SinkExecutor(l.Sink, output, sender))
	return line
}
