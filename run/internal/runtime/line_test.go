package runtime_test

import (
	"errors"
	"reflect"
	"testing"
)

var mockError = errors.New("mock error")

func TestLines(t *testing.T) {
	// ls := runtime.Lines{
	// 	Mutations: make(chan mutable.Mutations, 1),
	// 	Lines:     []runtime.Line{},
	// }
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
