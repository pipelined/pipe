package pool

import (
	"sync"

	"pipelined.dev/signal"
)

// Pool for signal buffers.
type Pool struct {
	bufferSize  int
	numChannels int
	pool        *sync.Pool
}

// New returns new pool.
func New(numChannels, bufferSize int) Pool {
	return Pool{
		bufferSize:  bufferSize,
		numChannels: numChannels,
		pool: &sync.Pool{
			New: func() interface{} {
				return signal.Float64Buffer(numChannels, bufferSize)
			},
		},
	}
}

// Alloc retrieves new signal.Float64 buffer from the pool.
func (p Pool) Alloc() signal.Float64 {
	return p.pool.Get().(signal.Float64)
}

// Free returns signal.Float64 buffer to the pool. Buffer is also cleared up.
func (p Pool) Free(b signal.Float64) {
	if b.NumChannels() == p.numChannels && b.Size() == p.bufferSize {
		for i := range b {
			for j := range b[i] {
				b[i][j] = 0
			}
		}
		p.pool.Put(b)
	}
}
