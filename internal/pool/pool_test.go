package pool_test

import (
	"testing"

	"pipelined.dev/pipe/internal/pool"
	"github.com/stretchr/testify/assert"
)

func TestPool(t *testing.T) {
	tests := []struct {
		numChannels int
		bufferSize  int
		allocs      int
	}{
		{
			numChannels: 1,
			bufferSize:  512,
			allocs:      10,
		},
		{
			numChannels: 100,
			bufferSize:  1024,
			allocs:      1000,
		},
	}
	for _, test := range tests {
		p := pool.New(test.numChannels, test.bufferSize)
		for i := 0; i < test.allocs; i++ {
			b := p.Alloc()
			assert.Equal(t, test.numChannels, b.NumChannels())
			assert.Equal(t, test.bufferSize, b.Size())
			p.Free(b)
		}
	}
}
