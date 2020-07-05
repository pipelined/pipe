package pooling_test

import (
	"testing"

	"pipelined.dev/pipe/pooling"

	"pipelined.dev/signal"
)

func TestPool(t *testing.T) {
	alloc := signal.Allocator{
		Channels: 10,
		Length:   512,
	}

	p1 := pooling.Get(alloc)
	p2 := pooling.Get(alloc)
	if p1 != p2 {
		t.Fatal("p1 must be equal to p2")
	}

	pooling.Wipe()
	p3 := pooling.Get(alloc)
	if p1 == p3 {
		t.Fatal("p1 must not be equal to p3")
	}
}
