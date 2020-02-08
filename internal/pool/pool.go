package pool

import (
	"sync"

	"pipelined.dev/signal/pool"
)

type key struct {
	bufferSize  int
	numChannels int
}

var m = struct {
	sync.Mutex
	pools map[key]*pool.Pool
}{
	pools: map[key]*pool.Pool{},
}

func Get(bufferSize, numChannels int) *pool.Pool {
	m.Lock()
	defer m.Unlock()
	k := key{bufferSize, numChannels}
	if p, ok := m.pools[k]; ok {
		return p
	}

	p := pool.New(numChannels, bufferSize)
	m.pools[k] = p
	return p
}
