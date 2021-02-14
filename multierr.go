package pipe

import "strings"

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
