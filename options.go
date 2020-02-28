package pipe

import (
	"pipelined.dev/pipe/mutate"
)

type Option func(*Pipe)

func WithRoutes(lines ...Line) Option {
	return func(p *Pipe) {
		for _, l := range lines {
			p.lines = append(p.lines, l)
			l.receivers(p.listeners)
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
