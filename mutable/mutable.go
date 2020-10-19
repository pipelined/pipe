package mutable

import (
	"crypto/rand"
)

// zero value for context is immutable.
var immutable = Context{}

type (
	// Context can be embedded to make structure behaviour mutable.
	Context [16]byte

	// Mutation is mutator function associated with a certain mutable context.
	Mutation struct {
		Context
		mutator MutatorFunc
	}

	// Mutations is a set of Mutations mapped their Mutables.
	Mutations map[Context][]MutatorFunc

	// MutatorFunc mutates the object.
	MutatorFunc func()
)

// Mutable returns new mutable mutable.
func Mutable() Context {
	var id [16]byte
	rand.Read(id[:])
	return id
}

// Immutable returns immutable mutable.
func Immutable() Context {
	return immutable
}

// Mutate associates provided mutator with mutable and return mutation.
func (c Context) Mutate(m MutatorFunc) Mutation {
	if c == immutable {
		panic("mutate immutable")
	}
	return Mutation{
		Context: c,
		mutator: m,
	}
}

// IsMutable returns true if object is mutable.
func (c Context) IsMutable() bool {
	return c != immutable
}

// Apply mutator function.
func (m Mutation) Apply() {
	m.mutator()
}

// Put mutation to the set of Mutations.
func (ms Mutations) Put(m Mutation) Mutations {
	if m.Context == immutable {
		return ms
	}
	if ms == nil {
		return map[Context][]MutatorFunc{m.Context: {m.mutator}}
	}

	if _, ok := ms[m.Context]; !ok {
		ms[m.Context] = []MutatorFunc{m.mutator}
	} else {
		ms[m.Context] = append(ms[m.Context], m.mutator)
	}

	return ms
}

// ApplyTo consumes Mutations defined for consumer in this param set.
func (ms Mutations) ApplyTo(id Context) {
	if ms == nil || id == immutable {
		return
	}
	if fns, ok := ms[id]; ok {
		for _, fn := range fns {
			fn()
		}
		delete(ms, id)
	}
}

// Append param set to another set.
func (ms Mutations) Append(source Mutations) Mutations {
	if ms == nil {
		ms = make(map[Context][]MutatorFunc)
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
func (ms Mutations) Detach(id Context) Mutations {
	if ms == nil {
		return nil
	}
	if v, ok := ms[id]; ok {
		d := map[Context][]MutatorFunc{id: v}
		delete(ms, id)
		return d
	}
	return nil
}
