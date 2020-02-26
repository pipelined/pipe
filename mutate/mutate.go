package mutate

import "crypto/rand"

var immutable = Mutability{}

type (
	// Mutability of the DSP line.
	Mutability [16]byte

	// Mutation is a set of mutators attached to a specific component.
	Mutation struct {
		Mutability
		Mutators []Mutator
		Puller   chan Mutators
	}
)

// Mutators is a set of mutators mapped to Receiver of their receivers.
// TODO: maybe this should not be exported
type Mutators map[[16]byte][]Mutator

type Mutator func() error

func (m Mutability) Mutate(ms ...Mutator) Mutation {
	return Mutation{
		Mutability: m,
		Mutators:   ms,
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

// Receiver allows to identify the component mutator belongs to.
// type Receiver [16]byte

// func NewReceiver() Receiver {
// 	var id [16]byte
// 	rand.Read(id[:])
// 	return id
// }

// Add appends a slice of Mutators.
func (m Mutators) Add(id [16]byte, fns ...Mutator) Mutators {
	if m == nil {
		return map[[16]byte][]Mutator{id: fns}
	}

	if _, ok := m[id]; !ok {
		m[id] = make([]Mutator, 0, len(fns))
	}
	m[id] = append(m[id], fns...)

	return m
}

// ApplyTo consumes Mutators defined for consumer in this param set.
func (m Mutators) ApplyTo(id [16]byte) error {
	if m == nil {
		return nil
	}
	if fns, ok := m[id]; ok {
		for _, fn := range fns {
			if err := fn(); err != nil {
				return err
			}
		}
		delete(m, id)
	}
	return nil
}

// Append param set to another set.
func (m Mutators) Append(source Mutators) Mutators {
	if m == nil {
		m = make(map[[16]byte][]Mutator)
	}
	for id, fns := range source {
		if _, ok := m[id]; ok {
			m[id] = append(m[id], fns...)
		} else {
			m[id] = fns
		}
	}
	return m
}

// Detach params for provided component id.
func (m Mutators) Detach(id [16]byte) Mutators {
	if m == nil {
		return nil
	}
	if v, ok := m[id]; ok {
		d := map[[16]byte][]Mutator{id: v}
		delete(m, id)
		return d
	}
	return nil
}
