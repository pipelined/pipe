package mutate

import (
	"crypto/rand"
)

var immutable = Mutability{}

type (
	// Mutability of the DSP line.
	Mutability [16]byte

	// Mutation is a set of mutators attached to a specific component.
	Mutation struct {
		Mutability
		mutator MutatorFunc
	}

	// Mutators is a set of mutators mapped to Receiver of their receivers.
	Mutators map[Mutability][]MutatorFunc

	MutatorFunc func() error
)

// Mutable returns new Mutability.
func Mutable() Mutability {
	var id [16]byte
	rand.Read(id[:])
	return id
}

func (m Mutability) Mutate(mutator MutatorFunc) Mutation {
	if m == immutable {
		return Mutation{}
	}
	return Mutation{
		Mutability: m,
		mutator:    mutator,
	}
}

func (m Mutability) Mutable() Mutability {
	if m == immutable {
		return Mutable()
	}
	return m
}

func (m Mutation) Apply() error {
	return m.mutator()
}

// Put mutation to the set of mutators.
func (ms Mutators) Put(m Mutation) Mutators {
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

// ApplyTo consumes Mutators defined for consumer in this param set.
func (ms Mutators) ApplyTo(id Mutability) error {
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
func (ms Mutators) Detach(id Mutability) Mutators {
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
