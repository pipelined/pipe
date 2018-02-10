package phono

import (
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"

	vst2 "github.com/dudk/vst2"
)

//Vst2 represents list of vst2 libraries
type Vst2 struct {
	Paths []string
	Libs  vst2Libraries
}

type vst2Libraries []vst2.Library

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
func NewVst2(paths []string) *Vst2 {
	vst := Vst2{}
	vst.Paths = append(vst.getDefaultScanPaths(), paths...)
	vst.Libs = vst2Libraries(make([]vst2.Library, 0))
	vst.Libs.Load(vst.Paths)
	return &vst
}

//Load vst2 libraries from defined paths
func (libraries *vst2Libraries) Load(paths []string) {
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

func (vst2 *Vst2) getDefaultScanPaths() (paths []string) {
	switch goos := runtime.GOOS; goos {
	case "darwin":
		paths = []string{
			"~/Library/Audio/Plug-Ins/VST",
			"/Library/Audio/Plug-Ins/VST",
		}
	case "windows":
		paths = []string{
			"C:\\Program Files (x86)\\Steinberg\\VSTPlugins",
			"C:\\Program Files\\Steinberg\\VSTPlugins ",
		}
		envVstPath := os.Getenv("VST_PATH")
		if len(envVstPath) > 0 {
			paths = append(paths, envVstPath)
		}
	}
	return
}
