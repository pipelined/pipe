package mutate

import (
	"crypto/rand"
)

// Mutators is a set of mutators mapped to Receiver of their receivers.
// TODO: maybe this should not be exported
type Mutators map[Receiver][]Mutator

type Mutator func() error

// Receiver allows to identify the component mutator belongs to.
type Receiver [16]byte

func NewReceiver() Receiver {
	var r [16]byte
	rand.Read(r[:])
	return r
}

// Add appends a slice of Mutators.
func (m Mutators) Add(r Receiver, fns ...Mutator) Mutators {
	if m == nil {
		return map[Receiver][]Mutator{r: fns}
	}

	if _, ok := m[r]; !ok {
		m[r] = make([]Mutator, 0, len(fns))
	}
	m[r] = append(m[r], fns...)

	return m
}

// ApplyTo consumes Mutators defined for consumer in this param set.
func (m Mutators) ApplyTo(r Receiver) error {
	if m == nil {
		return nil
	}
	if fns, ok := m[r]; ok {
		for _, fn := range fns {
			if err := fn(); err != nil {
				return err
			}
		}
		delete(m, r)
	}
	return nil
}

// Append param set to another set.
func (m Mutators) Append(source Mutators) Mutators {
	if m == nil {
		m = make(map[Receiver][]Mutator)
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
func (m Mutators) Detach(r Receiver) Mutators {
	if m == nil {
		return nil
	}
	if v, ok := m[r]; ok {
		d := map[Receiver][]Mutator{r: v}
		delete(m, r)
		return d
	}
	return nil
}
