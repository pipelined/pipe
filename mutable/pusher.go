package mutable

import "context"

type (
	// Pusher allows to push mutations to mutable contexts.
	Pusher struct {
		destinations map[Context]Destination
		mutations    map[Destination]Mutations
	}

	// Destination is a channel that used as source of mutations.
	Destination chan Mutations
)

// NewPusher creates new pusher.
func NewPusher() Pusher {
	return Pusher{
		destinations: make(map[Context]Destination),
		mutations:    make(map[Destination]Mutations),
	}
}

// AddDestination adds new mapping of mutable context to destination.
func (p Pusher) AddDestination(ctx Context, mc chan Mutations) {
	p.destinations[ctx] = mc
}

func NewDestination() Destination {
	return make(chan Mutations, 1)
}

// Destination returns destination if it's present in the map. Otherwise
// nil and false are returned.
// func (p Pusher) Destination(ctx Context) (d chan Mutations, ok bool) {
// 	if d, ok = p.destinations[ctx]; !ok {
// 		d = make(chan Mutations, 1)
// 	}
// 	return
// }

// Put mutations to the pusher. Function will panic if pusher contains
// unknown context.
func (p Pusher) Put(mutations ...Mutation) {
	for _, m := range mutations {
		if d, ok := p.destinations[m.Context]; ok {
			p.mutations[d] = p.mutations[d].Put(m)
			continue
		}
		panic("unknown mutable context")
	}
}

// Push mutations to the destinations.
func (p Pusher) Push(ctx context.Context) {
	for c, m := range p.mutations {
		if m != nil {
			select {
			case c <- m:
				p.mutations[c] = nil
			case <-ctx.Done():
				return
			}
		}
	}
}
