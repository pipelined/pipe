package pipe

import "pipelined.dev/pipe/mutability"

// Option represents pipe constructor parameter.
type Option func(*Pipe)

// WithLines provides lines for the pipe.
func WithLines(lines ...Line) Option {
	return func(p *Pipe) {
		for _, l := range lines {
			addLine(p, l)
		}
	}
}

// WithMutations provides mutations for the pipe. The mutations will be
// sent along the pipe with the first message.
func WithMutations(mutations ...mutability.Mutation) Option {
	return func(p *Pipe) {
		for _, m := range mutations {
			if c := p.listeners[m.Mutability]; c != nil {
				p.mutations[c] = p.mutations[c].Put(m)
			}
		}
	}
}
