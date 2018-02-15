package vst2

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	vst2 "github.com/dudk/vst2"
)

//Cache represents list of vst2 libraries
type Cache struct {
	Paths []string
	Libs  Libraries
}

//Libraries represent vst2 libs grouped by their path
type Libraries map[string][]vst2.Library

var (
	defaultScanPaths = getDefaultScanPaths()
	ext              = getExt()
)

//NewCache returns a slice of loaded vst2 libraries
func NewCache(paths ...string) *Cache {
	cache := Cache{}
	cache.Paths = uniquePaths(append(defaultScanPaths, paths...))
	cache.Load()
	return &cache
}

//Load vst2 libraries from defined paths
func (cache *Cache) Load() {
	cache.Libs = make(map[string][]vst2.Library)
	for _, path := range cache.Paths {
		cache.Libs[path] = make([]vst2.Library, 0)
		err := filepath.Walk(path, cache.loadLibs())
		if err != nil {
			log.Print(err)
		}
	}
}

//Close unloads all loaded libs
func (cache *Cache) Close() {
	for _, libs := range cache.Libs {
		for _, lib := range libs {
			lib.Close()
		}
	}
}

func (cache *Cache) loadLibs() filepath.WalkFunc {
	return func(path string, file os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return nil
		}
		if strings.HasSuffix(file.Name(), ext) {
			library, err := vst2.Open(path)
			if err != nil {
				return err
			}
			dir := filepath.Dir(path)
			cache.Libs[dir] = append(cache.Libs[dir], *library)
		}
		return nil
	}
}

func getDefaultScanPaths() (paths []string) {
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

func getExt() string {
	switch os := runtime.GOOS; os {
	case "darwin":
		return ".vst"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}

func uniquePaths(stringSlice []string) []string {
	u := make([]string, 0, len(stringSlice))
	m := make(map[string]bool)
	for _, val := range stringSlice {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}
	return u
}

func (cache Cache) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Scan paths:\n"))
	for _, path := range cache.Paths {
		buf.WriteString(fmt.Sprintf("\t%v\n", path))
	}
	buf.WriteString(fmt.Sprintf("Available plugins:\n"))
	buf.WriteString(fmt.Sprintf("%v", cache.Libs))
	return buf.String()
}

func (libraries Libraries) String() string {
	var buf bytes.Buffer
	for path, libs := range libraries {
		buf.WriteString(fmt.Sprintf("\t%v\n", path))
		if len(libs) == 0 {
			buf.WriteString(fmt.Sprintf("\t\t[No plugins found]\n"))
		}
		for _, lib := range libs {
			buf.WriteString(fmt.Sprintf("\t\t%v\n", lib.Name))
		}
	}
	return buf.String()
}

//LoadPlugin loads new plugin
func (cache *Cache) LoadPlugin(path string, name string) (*vst2.Plugin, error) {
	if len(cache.Libs) == 0 {
		return nil, fmt.Errorf("No plugins are found in folders %v", cache.Paths)
	}
	libs, ok := cache.Libs[path]
	if !ok {
		return nil, fmt.Errorf("Path %v was not scanned", path)
	}
	for _, lib := range libs {
		if lib.Name == name {
			return lib.Open()
		}
	}
	return nil, fmt.Errorf("Plugin %v not found at %v", name, path)
}
