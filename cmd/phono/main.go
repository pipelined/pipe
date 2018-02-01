package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/dudk/vst2"
)

var vstPaths []string

func main() {
	fmt.Printf("%v\n", vstPaths)
	plugins := vstPlugins(make([]vst2.Plugin, 0, 10))
	plugins.load(vstPaths)
	fmt.Println(plugins)
}

type vstPlugins []vst2.Plugin

//load vst plugins from defined paths
func (plugins *vstPlugins) load(paths []string) {
	for _, path := range paths {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Printf("Failed to read path: %v\n Error: %v\n", path, err)
			continue
		}
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".vst") {
				continue
			}
			plugin, err := vst2.LoadPlugin(path + "/" + file.Name())
			if err != nil {
				continue
			}
			*plugins = append(*plugins, *plugin)
		}
	}
}
