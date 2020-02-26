package mutate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pipelined.dev/pipe/mutate"
)

var id [16]byte

// paramMock used to set up test cases for mutators
type paramMock struct {
	value      int
	operations int
	expected   int
}

func (m *paramMock) mutators() mutate.Mutators {
	var p mutate.Mutators
	return p.Add(id, m.param())
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
		var mutators mutate.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Add(id, m.param())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(id)
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
		var mutators mutate.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Append(m.mutators())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(id)
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
		var mutators mutate.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Add(id, m.param())
			}
		}
		for _, m := range c.mocks {
			d := mutators.Detach(id)
			mutators.ApplyTo(id)
			assert.Equal(t, 0, m.value)
			d.ApplyTo(id)
			assert.Equal(t, m.expected, m.value)
		}
	}
}
