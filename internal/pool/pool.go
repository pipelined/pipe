package pool

import (
	"sync"

	"github.com/pipelined/signal"
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

func (p Pool) Alloc() signal.Float64 {
	return p.pool.Get().(signal.Float64)
}

func (p Pool) Free(b signal.Float64) {
	if b.NumChannels() == p.numChannels && b.Size() == p.bufferSize {
		p.pool.Put(b)
	}
}
