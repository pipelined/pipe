package mutate_test

import (
	"testing"

	"pipelined.dev/pipe/mutate"

	"github.com/stretchr/testify/assert"
)

// paramMock used to set up test cases for mutators
type paramMock struct {
	mutate.Mutability
	value      int
	operations int
	expected   int
}

func (m *paramMock) mutators() mutate.Mutators {
	var p mutate.Mutators
	return p.Put(m.param())
}

// mutators closure to mutate.value
func (m *paramMock) param() mutate.Mutation {
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
					Mutability: mutate.Mutable(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 2,
					expected:   20,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 3,
					expected:   30,
				},
				&paramMock{
					Mutability: mutate.Mutable(),
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
				mutators = mutators.Put(m.param())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(m.Mutability)
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
					operations: 10,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 2,
					expected:   20,
				},
				&paramMock{
					Mutability: mutate.Mutable(),
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
			mutators.ApplyTo(m.Mutability)
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
					operations: 10,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 2,
					expected:   20,
				},
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 3,
					expected:   30,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					Mutability: mutate.Mutable(),
					operations: 4,
					expected:   40,
				},
				&paramMock{
					Mutability: mutate.Mutable(),
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
				mutators = mutators.Put(m.param())
			}
		}
		for _, m := range c.mocks {
			d := mutators.Detach(m.Mutability)
			mutators.ApplyTo(m.Mutability)
			assert.Equal(t, 0, m.value)
			d.ApplyTo(m.Mutability)
			assert.Equal(t, m.expected, m.value)
		}
	}
}
