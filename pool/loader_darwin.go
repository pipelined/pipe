package pool

import (
	"io/ioutil"
	"log"
	"strings"

	vst2 "github.com/dudk/vst2"
)

//load vst2 plugins from defined paths
func (plugins *vst2Plugins) load(paths []string) {
	for _, path := range paths {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Printf("Failed to read path: %v\n Error: %v\n", path, err)
			continue
		}
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".vst") {
				if file.IsDir() {
					plugins.load([]string{path + "/" + file.Name()})
				}
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
