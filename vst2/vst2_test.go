package vst2_test

import (
	"testing"

	"github.com/dudk/phono/vst2"
	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	testPath := "../_testdata/vst2"
	cache := vst2.NewCache([]string{testPath})
	assert.NotNil(t, cache.Libs[testPath])
	assert.Equal(t, 1, len(cache.Libs[testPath]))
}
