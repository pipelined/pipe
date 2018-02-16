package vst2

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//Cache represents list of vst2 libraries
type Cache struct {
	Paths []string
	Libs  Libraries
}

//Libraries represent vst2 libs grouped by their path
type Libraries map[string][]Library

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
func (c *Cache) Load() {
	c.Libs = make(map[string][]Library)
	for _, path := range c.Paths {
		c.Libs[path] = make([]Library, 0)
		err := filepath.Walk(path, c.loadLibs())
		if err != nil {
			log.Print(err)
		}
	}
}

//Close unloads all loaded libs
func (c *Cache) Close() {
	for _, libs := range c.Libs {
		for _, lib := range libs {
			lib.Close()
		}
	}
}

func (c *Cache) loadLibs() filepath.WalkFunc {
	return func(path string, file os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return nil
		}
		if strings.HasSuffix(file.Name(), ext) {
			library, err := Open(path)
			if err != nil {
				return err
			}
			dir := filepath.Dir(path)
			c.Libs[dir] = append(c.Libs[dir], *library)
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

func (c Cache) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Scan paths:\n"))
	for _, path := range c.Paths {
		buf.WriteString(fmt.Sprintf("\t%v\n", path))
	}
	buf.WriteString(fmt.Sprintf("Available plugins:\n"))
	buf.WriteString(fmt.Sprintf("%v", c.Libs))
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
func (c *Cache) LoadPlugin(path string, name string) (*Plugin, error) {
	if len(c.Libs) == 0 {
		return nil, fmt.Errorf("No plugins are found in folders %v", c.Paths)
	}
	libs, ok := c.Libs[path]
	if !ok {
		return nil, fmt.Errorf("Path %v was not scanned", path)
	}
	for _, lib := range libs {
		if lib.Name == name {
			plugin, err := lib.Open()

			if err != nil {
				return nil, err
			}
			return plugin, nil
		}
	}
	return nil, fmt.Errorf("Plugin %v not found at %v", name, path)
}
