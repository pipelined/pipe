package mutable_test

import (
	"context"
	"testing"

	"pipelined.dev/pipe/mutable"
)

func TestPusher(t *testing.T) {
	p := mutable.NewPusher()
	ctx1 := mutable.Mutable()
	d, _ := p.Destination(ctx1)

	p.AddDestination(ctx1, d)
	var v int
	p.Put(ctx1.Mutate(func() error {
		v++
		return nil
	}))

	p.Push(context.Background())
	m := <-d
	m.ApplyTo(ctx1)

	assertEqual(t, "mutation ok", v, 1)
	assertPanic(t, func() {
		ctx2 := mutable.Mutable()
		p.Put(ctx2.Mutate(func() error { return nil }))
	})
}
