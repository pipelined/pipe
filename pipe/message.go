package pipe

import "github.com/dudk/phono"

// Message is a main structure for pipe transport
type message struct {
	// Buffer of message
	phono.Buffer
	// params for pipe
	*params
	// ID of original pipe
	sourceID string
}

// NewMessageFunc is a message-producer function
// sourceID expected to be pump's id
type newMessageFunc func() *message

// params represent a set of parameters mapped to ID of their receivers.
type params struct {
	private map[string][]phono.ParamFunc
}

// newParams returns a new params instance with initialised map inside.
func newParams(values ...phono.Param) *params {
	p := &params{
		private: make(map[string][]phono.ParamFunc),
	}
	p.add(values...)
	return p
}

// add appends a slice of params.
func (p *params) add(params ...phono.Param) *params {
	if p == nil {
		p = newParams(params...)
	} else {
		for _, param := range params {
			private, ok := p.private[param.ID]
			if !ok {
				private = make([]phono.ParamFunc, 0, len(params))
			}
			private = append(private, param.Apply)

			p.private[param.ID] = private
		}
	}

	return p
}

// applyTo consumes params defined for consumer in this param set.
func (p *params) applyTo(id string) {
	if p == nil {
		return
	}
	if params, ok := p.private[id]; ok {
		for _, param := range params {
			param()
		}
		delete(p.private, id)
	}
}

// merge two param sets into one.
func (p *params) merge(source *params) *params {
	if p == nil || p.empty() {
		return source
	}
	for newKey, newValues := range source.private {
		if _, ok := p.private[newKey]; ok {
			p.private[newKey] = append(p.private[newKey], newValues...)
		} else {
			p.private[newKey] = newValues
		}
	}
	return p
}

// empty returns true if params are empty.
func (p *params) empty() bool {
	if p == nil || p.private == nil || len(p.private) == 0 {
		return true
	}
	return false
}
