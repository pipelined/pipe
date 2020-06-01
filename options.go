package pipe

import (
	"pipelined.dev/pipe/mutate"
)

type Option func(*Pipe)

func WithLines(lines ...Line) Option {
	return func(p *Pipe) {
		for _, l := range lines {
			addLine(p, l)
		}
	}
}

func WithMutations(mutations ...mutate.Mutation) Option {
	return func(p *Pipe) {
		for _, m := range mutations {
			if c := p.listeners[m.Mutability]; c != nil {
				p.mutatorsByListeners[c] = p.mutatorsByListeners[c].Add(m.Mutability, m.Mutator)
			}
		}
	}
}
