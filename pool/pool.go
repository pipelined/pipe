/*
Package pool provides cache for signal pools.

The main use case for this package is to utilise pools with the same
allocators across multiple DSP components.
*/
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

// Get returns pool for provided allocator. Pools are cached internally, so
// multiple calls for same allocator will return the same pool instance.
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

// Wipe cleans up internal cache of pools.
func Wipe() {
	m.Lock()
	defer m.Unlock()
	m.pools = map[signal.Allocator]*signal.Pool{}
}
