package mutable_test

import (
	"testing"

	"pipelined.dev/pipe/mutable"

	"github.com/stretchr/testify/assert"
)

// paramMock used to set up test cases for mutators
type paramMock struct {
	mutable.Mutable
	value      int
	operations int
	expected   int
}

func (m *paramMock) mutators() mutable.Mutators {
	var p mutable.Mutators
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
		var mutators mutable.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Put(m.param())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(m.Mutable)
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
		var mutators mutable.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Append(m.mutators())
			}
		}
		for _, m := range c.mocks {
			mutators.ApplyTo(m.Mutable)
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
		var mutators mutable.Mutators
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Put(m.param())
			}
		}
		for _, m := range c.mocks {
			d := mutators.Detach(m.Mutable)
			mutators.ApplyTo(m.Mutable)
			assert.Equal(t, 0, m.value)
			d.ApplyTo(m.Mutable)
			assert.Equal(t, m.expected, m.value)
		}
	}
}
