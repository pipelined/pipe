package state

// Params represent a set of parameters mapped to ID of their receivers.
type Params map[string][]func()

// Add appends a slice of Params.
func (p Params) Add(componentID string, paramFuncs ...func()) Params {
	var private []func()
	if _, ok := p[componentID]; !ok {
		private = make([]func(), 0, len(paramFuncs))
	}
	private = append(private, paramFuncs...)

	p[componentID] = private
	return p
}

// ApplyTo consumes Params defined for consumer in this param set.
func (p Params) ApplyTo(id string) {
	if p == nil {
		return
	}
	if Params, ok := p[id]; ok {
		for _, param := range Params {
			param()
		}
		delete(p, id)
	}
}

// Merge two param sets into one.
func (p Params) Merge(source Params) Params {
	for newKey, newValues := range source {
		if _, ok := p[newKey]; ok {
			p[newKey] = append(p[newKey], newValues...)
		} else {
			p[newKey] = newValues
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
		d := Params(make(map[string][]func()))
		d[id] = v
		return d
	}
	return nil
}
