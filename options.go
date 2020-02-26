package pipe

import "pipelined.dev/pipe/mutate"

type Option func(*Pipe)

func WithRoutes(routes ...Route) Option {
	return func(p *Pipe) {
		for _, r := range routes {
			p.routes = append(p.routes, r)
		}
	}
}

func WithMutators(mutations ...mutate.Mutation) Option {
	return func(p *Pipe) {
		mutations = p.filterMutations(mutations)
		for _, m := range mutations {
			p.mutators[m.Puller] = p.mutators[m.Puller].Add(m.Mutability, m.Mutators...)
		}
	}
}
