package pool_test

import (
	"testing"

	"pipelined.dev/pipe/pool"
	"pipelined.dev/signal"
)

func TestPool(t *testing.T) {
	alloc := signal.Allocator{
		Channels: 10,
		Length:   512,
	}

	p1 := pool.Get(alloc)
	p2 := pool.Get(alloc)
	if p1 != p2 {
		t.Fatal("p1 must be equal to p2")
	}

	pool.Wipe()
	p3 := pool.Get(alloc)
	if p1 == p3 {
		t.Fatal("p1 must not be equal to p3")
	}
}
