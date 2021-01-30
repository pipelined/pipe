package mutable_test

import (
	"reflect"
	"testing"

	"pipelined.dev/pipe/mutable"
)

// mutableMock used to set up test cases for mutators
type mutableMock struct {
	Mutability mutable.Context
	value      int
	operations int
	expected   int
}

// mutators closure to mutable.value
func (m *mutableMock) AddDelta(delta int) mutable.Mutation {
	return m.Mutability.Mutate(func() error {
		m.value += delta
		return nil
	})
}

func TestPutMutations(t *testing.T) {
	var tests = []struct {
		mocks []*mutableMock
	}{
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 2,
					expected:   20,
				},
			},
		},
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 3,
					expected:   30,
				},
				{
					Mutability: mutable.Mutable(),
					operations: 4,
					expected:   40,
				},
			},
		},
	}

	for _, c := range tests {
		var mutations mutable.Mutations
		delta := 10
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutations = mutations.Put(m.AddDelta(delta))
			}
		}
		for _, m := range c.mocks {
			mutations.ApplyTo(m.Mutability)
			assertEqual(t, "value", m.value, m.expected)
			assertEqual(t, "mutability", m.Mutability.IsMutable(), true)
		}
	}
}

func TestAppendMutations(t *testing.T) {
	var tests = []struct {
		mocks []*mutableMock
	}{
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 2,
					expected:   20,
				},
				{
					Mutability: mutable.Mutable(),
					operations: 3,
					expected:   30,
				},
			},
		},
	}

	for _, c := range tests {
		var mutations mutable.Mutations
		delta := 10
		for _, m := range c.mocks {
			var mockMutations mutable.Mutations
			for j := 0; j < m.operations; j++ {
				mutations = mutations.Append(mockMutations.Put(m.AddDelta(delta)))
			}
		}
		for _, m := range c.mocks {
			mutations.ApplyTo(m.Mutability)
			assertEqual(t, "value", m.value, m.expected)
		}
	}
}

func TestDetachMutations(t *testing.T) {
	var tests = []struct {
		mocks []*mutableMock
	}{
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 1,
					expected:   10,
				},
			},
		},
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 2,
					expected:   20,
				},
				{
					Mutability: mutable.Mutable(),
					operations: 3,
					expected:   30,
				},
			},
		},
		{
			mocks: []*mutableMock{
				{
					Mutability: mutable.Mutable(),
					operations: 4,
					expected:   40,
				},
				{
					Mutability: mutable.Mutable(),
					operations: 0,
					expected:   0,
				},
			},
		},
	}

	for _, c := range tests {
		var mutators mutable.Mutations
		delta := 10
		for _, m := range c.mocks {
			for j := 0; j < m.operations; j++ {
				mutators = mutators.Put(m.AddDelta(delta))
			}
		}
		for _, m := range c.mocks {
			d := mutators.Detach(m.Mutability)
			mutators.ApplyTo(m.Mutability)
			assertEqual(t, "value before", m.value, 0)
			d.ApplyTo(m.Mutability)
			assertEqual(t, "value after", m.value, m.expected)
		}
	}
}

func TestMutability(t *testing.T) {
	mut := mutable.Immutable()
	assertEqual(t, "immutable", mut.IsMutable(), false)
	mut = mutable.Mutable()
	assertEqual(t, "mutable", mut.IsMutable(), true)
	assertPanic(t, func() {
		mutable.Immutable().Mutate(func() error {
			return nil
		})
	})
	mock := &mutableMock{
		Mutability: mutable.Mutable(),
	}
	delta := 10
	mock.AddDelta(delta).Apply()
	assertEqual(t, "apply", mock.value, delta)
}

func assertEqual(t *testing.T, name string, result, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("%v\nresult: \t%T\t%+v \nexpected: \t%T\t%+v", name, result, result, expected, expected)
	}
}

func assertPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	fn()
}
