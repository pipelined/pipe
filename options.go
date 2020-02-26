package pipe

type Option func(*Pipe)

func WithRoutes(routes ...Route) Option {
	return func(p *Pipe) {
		for _, r := range routes {
			p.routes = append(p.routes, r)
		}
	}
}

func WithMutators(mutations ...Mutation) Option {
	return func(p *Pipe) {
		mutations = p.filterMutations(mutations)
		for _, m := range mutations {
			p.mutators[m.puller] = p.mutators[m.puller].Add(m.Mutability, m.Mutators...)
		}
	}
}
