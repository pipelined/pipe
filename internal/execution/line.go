package execution

import (
	"context"
	"strings"
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
