package mutator

// Mutators is a set of mutators mapped to Receiver of their receivers.
type Mutators map[*Receiver][]func()

// Receiver allows to identify the component mutator belongs to.
type Receiver struct{}

// Add appends a slice of Mutators.
func (m Mutators) Add(r *Receiver, fns ...func()) Mutators {
	if m == nil {
		return map[*Receiver][]func(){r: fns}
	}

	if _, ok := m[r]; !ok {
		m[r] = make([]func(), 0, len(fns))
	}
	m[r] = append(m[r], fns...)

	return m
}

// ApplyTo consumes Mutators defined for consumer in this param set.
func (m Mutators) ApplyTo(r *Receiver) {
	if m == nil {
		return
	}
	if fns, ok := m[r]; ok {
		for _, fn := range fns {
			fn()
		}
		delete(m, r)
	}
}

// Append param set to another set.
func (m Mutators) Append(source Mutators) Mutators {
	if m == nil {
		m = make(map[*Receiver][]func())
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
func (m Mutators) Detach(r *Receiver) Mutators {
	if m == nil {
		return nil
	}
	if v, ok := m[r]; ok {
		d := map[*Receiver][]func(){r: v}
		delete(m, r)
		return d
	}
	return nil
}
