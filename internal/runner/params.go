package runner

// Params represent a set of parameters mapped to ID of their receivers.
type Params map[string][]func()

// Add appends a slice of Params.
func (p Params) Add(id string, fns ...func()) Params {
	if p == nil {
		return map[string][]func(){id: fns}
	}

	if _, ok := p[id]; !ok {
		p[id] = make([]func(), 0, len(fns))
	}
	p[id] = append(p[id], fns...)

	return p
}

// ApplyTo consumes Params defined for consumer in this param set.
func (p Params) ApplyTo(id string) {
	if p == nil {
		return
	}
	if fns, ok := p[id]; ok {
		for _, fn := range fns {
			fn()
		}
		delete(p, id)
	}
}

// Append param set to another set.
func (p Params) Append(source Params) Params {
	if p == nil {
		p = make(map[string][]func())
	}
	for id, fns := range source {
		if _, ok := p[id]; ok {
			p[id] = append(p[id], fns...)
		} else {
			p[id] = fns
		}
	}
	return p
}

// Detach params for provided component id.
func (p Params) Detach(id string) Params {
	if p == nil {
		return nil
	}
	if v, ok := p[id]; ok {
		d := map[string][]func(){id: v}
		delete(p, id)
		return d
	}
	return nil
}
