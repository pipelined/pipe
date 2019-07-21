package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pipelined/pipe/internal/state"
)

// paramMock used to set up test cases for params
type paramMock struct {
	uid        string
	value      int
	operations int
	expected   int
}

func (m *paramMock) params() state.Params {
	var p state.Params
	return p.Add(m.uid, m.param())
}

// params closure to mutate value
func (m *paramMock) param() func() {
	return func() {
		m.value += 10
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
					uid:        "1",
					operations: 0,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					uid:        "1",
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					uid:        "1",
					operations: 2,
					expected:   20,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					uid:        "1",
					operations: 3,
					expected:   30,
				},
				&paramMock{
					uid:        "2",
					operations: 4,
					expected:   40,
				},
			},
		},
	}

	for _, c := range tests {
		var params state.Params
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				params = params.Add(m.uid, m.param())
			}
		}
		for _, m := range c.mocks {
			params.ApplyTo(m.uid)
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
					uid:        "1",
					operations: 0,
					expected:   0,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					uid:        "1",
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*paramMock{
				&paramMock{
					uid:        "1",
					operations: 2,
					expected:   20,
				},
				&paramMock{
					uid:        "2",
					operations: 3,
					expected:   30,
				},
			},
		},
	}

	for _, c := range tests {
		var params state.Params
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				params = params.Append(m.params())
			}
		}
		for _, m := range c.mocks {
			params.ApplyTo(m.uid)
			assert.Equal(t, m.expected, m.value)
		}
	}
}
