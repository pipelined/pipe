// Package mutability provides types to make DSP components mutable.
package mutability

import (
	"crypto/rand"
)

// zero value for mutability is immutable.
var immutable = Mutability{}

type (
	// Mutability can be embedded to make structure behaviour mutability.
	Mutability [16]byte

	// Mutation is mutator function associated with a certain mutability.
	Mutation struct {
		Mutability
		mutator MutatorFunc
	}

	// Mutations is a set of Mutations mapped their Mutables.
	Mutations map[Mutability][]MutatorFunc

	// MutatorFunc mutates the object.
	MutatorFunc func() error
)

// Mutable returns new mutable Mutability.
func Mutable() Mutability {
	var id [16]byte
	rand.Read(id[:])
	return id
}

// Immutable returns immutable Mutability.
func Immutable() Mutability {
	return immutable
}

// Mutate associates provided mutator with mutable and return mutation.
func (m Mutability) Mutate(mutator MutatorFunc) Mutation {
	if m == immutable {
		panic("mutate immutable")
	}
	return Mutation{
		Mutability: m,
		mutator:    mutator,
	}
}

// Immutable returns true if object is immutable.
func (m Mutability) Immutable() bool {
	return m == immutable
}

// Apply mutator function.
func (m Mutation) Apply() error {
	return m.mutator()
}

// Put mutation to the set of Mutations.
func (ms Mutations) Put(m Mutation) Mutations {
	if m.Mutability == immutable {
		return ms
	}
	if ms == nil {
		return map[Mutability][]MutatorFunc{m.Mutability: {m.mutator}}
	}

	if _, ok := ms[m.Mutability]; !ok {
		ms[m.Mutability] = []MutatorFunc{m.mutator}
	} else {
		ms[m.Mutability] = append(ms[m.Mutability], m.mutator)
	}

	return ms
}

// ApplyTo consumes Mutations defined for consumer in this param set.
func (ms Mutations) ApplyTo(id Mutability) error {
	if ms == nil || id == immutable {
		return nil
	}
	if fns, ok := ms[id]; ok {
		for _, fn := range fns {
			if err := fn(); err != nil {
				return err
			}
		}
		delete(ms, id)
	}
	return nil
}

// Append param set to another set.
func (ms Mutations) Append(source Mutations) Mutations {
	if ms == nil {
		ms = make(map[Mutability][]MutatorFunc)
	}
	for id, fns := range source {
		if _, ok := ms[id]; ok {
			ms[id] = append(ms[id], fns...)
		} else {
			ms[id] = fns
		}
	}
	return ms
}

// Detach params for provided component id.
func (ms Mutations) Detach(id Mutability) Mutations {
	if ms == nil {
		return nil
	}
	if v, ok := ms[id]; ok {
		d := map[Mutability][]MutatorFunc{id: v}
		delete(ms, id)
		return d
	}
	return nil
}
