package pool

import (
	"sync"

	"pipelined.dev/signal"
)

var m = struct {
	sync.Mutex
	pools map[signal.Allocator]*signal.Pool
}{
	pools: map[signal.Allocator]*signal.Pool{},
}

func Get(allocator signal.Allocator) *signal.Pool {
	m.Lock()
	defer m.Unlock()
	if p, ok := m.pools[allocator]; ok {
		return p
	}

	p := allocator.Pool()
	m.pools[allocator] = p
	return p
}

func Prune() {
	m.Lock()
	defer m.Unlock()
	m.pools = map[signal.Allocator]*signal.Pool{}
}
