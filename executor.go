package pipe

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type execErrors []error

func (e execErrors) Error() string {
	s := []string{}
	for _, se := range e {
		s = append(s, se.Error())
	}
	return strings.Join(s, ",")
}

// ret returns untyped nil if error is list is empty.
func (e execErrors) ret() error {
	if len(e) > 0 {
		return e
	}
	return nil
}

// Start calls start for every line. If any line fails to start, it will
// try to flush successfully started lines.
func (e executor) Start(ctx context.Context) error {
	var startErr execErrors
	for _, ex := range e.Executors {
		if err := ex.Start(ctx); err != nil {
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
		err = fmt.Errorf("error flushing lines: %w during start error: %v", flushErr, err)
	}
	return err
}

// Flush flushes all lines.
func (e executor) Flush(ctx context.Context) error {
	var flushErr execErrors
	for _, ex := range e.Executors {
		if err := ex.Flush(ctx); err != nil {
			flushErr = append(flushErr, err)
		}
	}
	return flushErr.ret()
}

// Execute executes all lines.
func (e executor) Execute(ctx context.Context) error {
	var err error
	for i := 0; i < len(e.Executors); {
		if err = e.Executors[i].Execute(ctx); err == nil {
			i++
			continue
		}
		if err == io.EOF {
			if flushErr := e.Executors[i].Flush(ctx); flushErr != nil {
				return flushErr
			}
			e.Executors = append(e.Executors[:i], e.Executors[i+1:]...)
			if len(e.Executors) > 0 {
				continue
			}
		}
		return err
	}
	return nil
}
