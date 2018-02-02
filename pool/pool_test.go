package pool_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dudk/phono/pool"
)

func TestPool(t *testing.T) {
	paths := []string{"../_testdata"}

	pool.New(paths)

	assert.NotNil(t, pool.Get())
	assert.Equal(t, len(pool.Get().Vst2), 1)
}
