package pool

import (
	"sync"

	vst2 "github.com/dudk/vst2"
)

//Pool holds the lists of available plugins
type Pool struct {
	Vst2 vst2Plugins
}

type vst2Plugins []vst2.Plugin

var instance *Pool
var once sync.Once

//New initialises a pool of plugins
func New(vstPaths []string) {
	once.Do(func() {
		vst2Plugins := vst2Plugins(make([]vst2.Plugin, 0))
		vst2Plugins.load(vstPaths)
		instance = &Pool{
			Vst2: vst2Plugins,
		}
	})
}

//Get returns pool singleton
func Get() *Pool {
	return instance
}
