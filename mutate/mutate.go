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
