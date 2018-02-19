package cache_test

import (
	"testing"

	"github.com/dudk/phono/cache"
	"github.com/stretchr/testify/assert"
)

var (
	vstPath = "../_testdata/vst2"
	vstName = "Krush"
)

func TestCache(t *testing.T) {
	cache := cache.NewVST2(vstPath)
	defer cache.Close()
	assert.NotNil(t, cache.Libs[vstPath])
	assert.Equal(t, 1, len(cache.Libs[vstPath]))
	plugin, err := cache.LoadPlugin(vstPath, vstName)
	defer plugin.Close()
	assert.Nil(t, err)
	assert.NotNil(t, plugin)
	assert.Equal(t, vstName, plugin.Name)
}
