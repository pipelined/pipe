package mutable_test

import (
	"reflect"
	"testing"

	"pipelined.dev/pipe/mutable"
)

// paramMock used to set up test cases for mutators
type paramMock struct {
	mutable.Mutable
	value      int
	operations int
	expected   int
}

func (m *paramMock) mutators() mutable.Mutations {
	var p mutable.Mutations
	return p.Put(m.param())
}

// mutators closure to mutable.value
func (m *paramMock) param() mutable.Mutation {
	return m.Mutate(func() error {
		m.value += 10
		return nil
	})
}

func TestAddParams(t *testing.T) {
	var tests = []struct {
		mocks []*paramMock
	}{
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 10,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 2,
					expected:   20,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 3,
					expected:   30,
				},
				&paramMock{
					Mutable:    mutable.New(),
					operations: 4,
					expected:   40,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators mutable.Mutations
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Put(m.param())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(m.Mutable)
			assertEqual(t, "value", m.value, m.expected)
		}
	}
}

func TestAppendParams(t *testing.T) {
	var tests = []struct {
		mocks []*paramMock
	}{
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 10,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 2,
					expected:   20,
				},
				&paramMock{
					Mutable:    mutable.New(),
					operations: 3,
					expected:   30,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators mutable.Mutations
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Append(m.mutators())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(m.Mutable)
			assertEqual(t, "value", m.value, m.expected)
		}
	}
}

func TestDetachParams(t *testing.T) {
	var tests = []struct {
		mocks []*paramMock
	}{
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 10,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 2,
					expected:   20,
				},
				&paramMock{
					Mutable:    mutable.New(),
					operations: 3,
					expected:   30,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutable:    mutable.New(),
					operations: 4,
					expected:   40,
				},
				&paramMock{
					Mutable:    mutable.New(),
					operations: 0,
					expected:   0,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators mutable.Mutations
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Put(m.param())
			}
		}
		for _, m := range c.mocks {
			d := mutators.Detach(m.Mutable)
			mutators.ApplyTo(m.Mutable)
			assertEqual(t, "value before", m.value, 0)
			d.ApplyTo(m.Mutable)
			assertEqual(t, "value after", m.value, m.expected)
		}
	}
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}
