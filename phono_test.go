package phono_test

import (
	"sync"
	"testing"

	"github.com/pipelined/phono"
	"github.com/stretchr/testify/assert"
)

func TestSingleUse(t *testing.T) {
	var once sync.Once
	err := phono.SingleUse(&once)
	assert.Nil(t, err)
	err = phono.SingleUse(&once)
	assert.Equal(t, phono.ErrSingleUseReused, err)
}
