package mutable

import (
	"crypto/rand"
)

var immutable = Mutable{}

type (
	// Mutable can be embedded to make structure behaviour mutable.
	Mutable [16]byte

	// Mutation is a set of mutators attached to a specific component.
	Mutation struct {
		Mutable
		mutator MutatorFunc
	}

	// Mutators is a set of mutators mapped to Receiver of their receivers.
	Mutators map[Mutable][]MutatorFunc

	MutatorFunc func() error
)

// New returns new Mutable.
func New() Mutable {
	var id [16]byte
	rand.Read(id[:])
	return id
}

func (m Mutable) Mutate(mutator MutatorFunc) Mutation {
	if m == immutable {
		return Mutation{}
	}
	return Mutation{
		Mutable: m,
		mutator: mutator,
	}
}

func (m Mutation) Apply() error {
	return m.mutator()
}

// Put mutation to the set of mutators.
func (ms Mutators) Put(m Mutation) Mutators {
	if m.Mutable == immutable {
		return ms
	}
	if ms == nil {
		return map[Mutable][]MutatorFunc{m.Mutable: {m.mutator}}
	}

	if _, ok := ms[m.Mutable]; !ok {
		ms[m.Mutable] = []MutatorFunc{m.mutator}
	} else {
		ms[m.Mutable] = append(ms[m.Mutable], m.mutator)
	}

	return ms
}

// ApplyTo consumes Mutators defined for consumer in this param set.
func (ms Mutators) ApplyTo(id Mutable) error {
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
func (ms Mutators) Append(source Mutators) Mutators {
	if ms == nil {
		ms = make(map[Mutable][]MutatorFunc)
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
func (ms Mutators) Detach(id Mutable) Mutators {
	if ms == nil {
		return nil
	}
	if v, ok := ms[id]; ok {
		d := map[Mutable][]MutatorFunc{id: v}
		delete(ms, id)
		return d
	}
	return nil
}
