package runner_test

import (
	"testing"

	"pipelined.dev/pipe/mutate"

	"github.com/stretchr/testify/assert"
	"pipelined.dev/pipe/internal/runner"
)

var id [16]byte

// paramMock used to set up test cases for mutators
type paramMock struct {
	id         [16]byte
	value      int
	operations int
	expected   int
}

func (m *paramMock) mutators() runner.Mutators {
	var p runner.Mutators
	return p.Add(id, m.param())
}

// mutators closure to runner.value
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
		var mutators runner.Mutators
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
		var mutators runner.Mutators
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
					id:         mutate.Mutable(),
					operations: 0,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					id:         mutate.Mutable(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					id:         mutate.Mutable(),
					operations: 2,
					expected:   20,
				},
				&paramMock{
					id:         mutate.Mutable(),
					operations: 3,
					expected:   30,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					id:         mutate.Mutable(),
					operations: 4,
					expected:   40,
				},
				&paramMock{
					id:         mutate.Mutable(),
					operations: 0,
					expected:   0,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators runner.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Add(m.id, m.param())
			}
		}
		for _, m := range c.mocks {
			d := mutators.Detach(m.id)
			mutators.ApplyTo(m.id)
			assert.Equal(t, 0, m.value)
			d.ApplyTo(m.id)
			assert.Equal(t, m.expected, m.value)
		}
	}
}
