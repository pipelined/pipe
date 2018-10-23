package pipe

import "github.com/dudk/phono"

// Message is a main structure for pipe transport
type message struct {
	phono.Buffer        // Buffer of message
	params              // params for pipe
	sourceID     string // ID of pipe which spawned this message.
	feedback     params //feedback are params applied after processing happened
}

// NewMessageFunc is a message-producer function
// sourceID expected to be pump's id
type newMessageFunc func() message

// params represent a set of parameters mapped to ID of their receivers.
type params map[string][]phono.ParamFunc

// add appends a slice of params.
func (p params) add(params ...phono.Param) params {
	for _, param := range params {
		private, ok := p[param.ID]
		if !ok {
			private = make([]phono.ParamFunc, 0, len(params))
		}
		private = append(private, param.Apply)

		p[param.ID] = private
	}
	return p
}

// applyTo consumes params defined for consumer in this param set.
func (p params) applyTo(id string) {
	if p == nil {
		return
	}
	if params, ok := p[id]; ok {
		for _, param := range params {
			param()
		}
		delete(p, id)
	}
}

// merge two param sets into one.
func (p params) merge(source params) params {
	for newKey, newValues := range source {
		if _, ok := p[newKey]; ok {
			p[newKey] = append(p[newKey], newValues...)
		} else {
			p[newKey] = newValues
		}
	}
	return p
}
