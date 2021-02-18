package pipe

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorRun is returned if runner was successfully started, but execution
// and/or flush failed.
type ErrorRun struct {
	ErrExec  error
	ErrFlush error
}

func (e *ErrorRun) Error() string {
	switch {
	case e.ErrExec != nil && e.ErrFlush != nil:
		return fmt.Sprintf("flush error: %v after execute error: %v", e.ErrFlush, e.ErrExec)
	case e.ErrExec != nil:
		return fmt.Sprintf("execute error: %v", e.ErrFlush)
	case e.ErrFlush != nil:
		return fmt.Sprintf("flush error: %v", e.ErrFlush)
	}
	return ""
}

// Is checks if any of errors match provided sentinel error.q
func (e *ErrorRun) Is(err error) bool {
	if e.ErrExec != nil && errors.Is(e.ErrExec, err) {
		return true
	}
	if e.ErrFlush != nil && errors.Is(e.ErrFlush, err) {
		return true
	}
	return false
}

// execErrors wraps errors that might occure when multiple executors
// are failing.
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
