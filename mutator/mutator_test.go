package mutator_test

import (
	"testing"

	"pipelined.dev/pipe/mutator"

	"github.com/stretchr/testify/assert"
)

// paramMock used to set up test cases for mutators
type paramMock struct {
	mutator.Receiver
	value      int
	operations int
	expected   int
}

func (m *paramMock) mutators() mutator.Mutators {
	var p mutator.Mutators
	return p.Add(&m.Receiver, m.param())
}

// mutators closure to mutate value
func (m *paramMock) param() func() error {
	return func() error {
		m.value += 10
		return nil
	}
}

func TestAddParams(t *testing.T) {
	var tests = []struct {
		mocks []*paramMock
	}{
		{},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 0,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 2,
					expected:   20,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 3,
					expected:   30,
				},
				&paramMock{
					operations: 4,
					expected:   40,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators mutator.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Add(&m.Receiver, m.param())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(&m.Receiver)
			assert.Equal(t, m.expected, m.value)
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
					operations: 0,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 2,
					expected:   20,
				},
				&paramMock{
					operations: 3,
					expected:   30,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators mutator.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Append(m.mutators())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(&m.Receiver)
			assert.Equal(t, m.expected, m.value)
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
					operations: 0,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 2,
					expected:   20,
				},
				&paramMock{
					operations: 3,
					expected:   30,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					operations: 4,
					expected:   40,
				},
				&paramMock{
					operations: 0,
					expected:   0,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators mutator.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Add(&m.Receiver, m.param())
			}
		}
		for _, m := range c.mocks {
			d := mutators.Detach(&m.Receiver)
			mutators.ApplyTo(&m.Receiver)
			assert.Equal(t, 0, m.value)
			d.ApplyTo(&m.Receiver)
			assert.Equal(t, m.expected, m.value)
		}
	}
}
