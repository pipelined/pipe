package pipe

import "github.com/dudk/phono"

// Message is a main structure for pipe transport
type message struct {
	// Buffer of message
	phono.Buffer
	// Params for pipe
	*Params
	// ID of original pipe
	SourceID string
}

// NewMessageFunc is a message-producer function
// sourceID expected to be pump's id
type newMessageFunc func() *message

// Params represent a set of parameters mapped to ID of their receivers
type Params struct {
	private map[string][]phono.ParamFunc
}

// NewParams returns a new params instance with initialised map inside
func NewParams(params ...phono.Param) (result *Params) {
	result = &Params{
		private: make(map[string][]phono.ParamFunc),
	}
	result.Add(params...)
	return
}

// Add accepts a slice of params
func (p *Params) Add(params ...phono.Param) *Params {
	if p == nil {
		p = NewParams(params...)
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

// ApplyTo consumes params defined for consumer in this param set
func (p *Params) ApplyTo(id string) {
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

// Merge two param sets into one
func (p *Params) Merge(source *Params) *Params {
	if p == nil || p.Empty() {
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

// Empty returns true if params are empty
func (p *Params) Empty() bool {
	if p == nil || p.private == nil || len(p.private) == 0 {
		return true
	}
	return false
}
