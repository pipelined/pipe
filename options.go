package pipe

import (
	"pipelined.dev/pipe/mutate"
)

type Option func(*Pipe)

func WithRoutes(routes ...Route) Option {
	return func(p *Pipe) {
		for _, r := range routes {
			p.routes = append(p.routes, r)
			r.receivers(p.receivers)
		}
	}
}

func WithMutators(mutations ...mutate.Mutation) Option {
	return func(p *Pipe) {
		for _, m := range mutations {
			if puller := p.receivers[m.Mutability]; puller != nil {
				p.mutators[puller] = p.mutators[puller].Add(m.Mutability, m.Mutator)
			}
		}
	}
}
