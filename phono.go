package phono

import (
	"io/ioutil"
	"log"
	"runtime"
	"strings"

	vst2 "github.com/dudk/vst2"
)

//Vst2 represents list of vst2 libraries
type Vst2 []vst2.Library

var (
	vst2ext = getVst2Ext()
)

func getVst2Ext() string {
	switch os := runtime.GOOS; os {
	case "darwin":
		return ".vst"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}

//NewVst2 returns a slice of loaded vst2 libraries
func NewVst2(paths []string) Vst2 {
	result := Vst2(make([]vst2.Library, 0))
	result.Load(paths)
	return result
}

//Load vst2 libraries from defined paths
func (libraries *Vst2) Load(paths []string) {
	for _, path := range paths {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Printf("Failed to read path: %v\n Error: %v\n", path, err)
			continue
		}
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), vst2ext) {
				if file.IsDir() {
					libraries.Load([]string{path + "/" + file.Name()})
				}
				continue
			}
			library, err := vst2.Open(path + "/" + file.Name())
			if err != nil {
				continue
			}
			*libraries = append(*libraries, *library)
		}
	}
}
