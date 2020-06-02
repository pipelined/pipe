package mutate

import "crypto/rand"

var immutable = Mutability{}

type (
	// Mutability of the DSP line.
	Mutability [16]byte

	// Mutation is a set of mutators attached to a specific component.
	Mutation struct {
		Mutability
		Mutator func() error
	}

	// Mutators is a set of mutators mapped to Receiver of their receivers.
	Mutators map[Mutability][]func() error
)

func (m Mutability) Mutate(mutator func() error) Mutation {
	if m == immutable {
		return Mutation{}
	}
	return Mutation{
		Mutability: m,
		Mutator:    mutator,
	}
}

func (m Mutability) Mutable() Mutability {
	if m == immutable {
		return Mutable()
	}
	return m
}

func Mutable() Mutability {
	var id [16]byte
	rand.Read(id[:])
	return id
}

// Add appends a slice of Mutators.
func (ms Mutators) Add(id Mutability, fns ...func() error) Mutators {
	if ms == nil {
		return map[Mutability][]func() error{id: fns}
	}

	if _, ok := ms[id]; !ok {
		ms[id] = make([]func() error, 0, len(fns))
	}
	ms[id] = append(ms[id], fns...)

	return ms
}

// ApplyTo consumes Mutators defined for consumer in this param set.
func (ms Mutators) ApplyTo(id Mutability) error {
	if ms == nil {
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
		ms = make(map[Mutability][]func() error)
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
func (ms Mutators) Detach(id [16]byte) Mutators {
	if ms == nil {
		return nil
	}
	if v, ok := ms[id]; ok {
		d := map[Mutability][]func() error{id: v}
		delete(ms, id)
		return d
	}
	return nil
}
