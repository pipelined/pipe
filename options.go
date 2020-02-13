package pipe

type Option func(*Pipe)

func WithRoutes(routes ...Route) Option {
	return func(p *Pipe) {
		for _, r := range routes {
			p.routes[r.mutators] = r
		}
	}
}

func WithMutators(mutations ...Mutation) Option {
	return func(p *Pipe) {
		for _, m := range mutations {
			p.mutators[m.Handle.puller] = p.mutators[m.Handle.puller].Add(m.Handle.receiver, m.Mutators...)
		}
	}
}
