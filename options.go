package pipe

import "pipelined.dev/pipe/mutability"

type Option func(*Pipe)

func WithLines(lines ...Line) Option {
	return func(p *Pipe) {
		for _, l := range lines {
			addLine(p, l)
		}
	}
}

func WithMutations(mutations ...mutability.Mutation) Option {
	return func(p *Pipe) {
		for _, m := range mutations {
			if c := p.listeners[m.Mutability]; c != nil {
				p.mutatorsByListeners[c] = p.mutatorsByListeners[c].Put(m)
			}
		}
	}
}
