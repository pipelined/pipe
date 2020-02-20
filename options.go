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
			p.mutators[m.Component.puller] = p.mutators[m.Component.puller].Add(m.Component.receiver, m.Mutators...)
		}
	}
}
