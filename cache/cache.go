package cache

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dudk/phono/vst2"
)

// VST2 represents list of vst2 libraries
type VST2 struct {
	Paths []string
	Libs  vst2libraries
}

// Libraries represent vst2 libs grouped by their path
type vst2libraries map[string]map[string]vst2.Library

var (
	vst2ScanPaths = vst2.DefaultScanPaths()
	vst2ext       = vst2.FileExtension()
)

// NewVST2 returns a slice of loaded vst2 libraries
func NewVST2(paths ...string) *VST2 {
	vst2 := VST2{}
	vst2.Paths = uniquePaths(append(vst2ScanPaths, paths...))
	vst2.load()
	return &vst2
}

// Load vst2 libraries from defined paths
func (v *VST2) load() {
	v.Libs = make(map[string]map[string]vst2.Library)
	for _, path := range v.Paths {
		err := filepath.Walk(path, v.loadLibs())
		if err != nil {
			log.Print(err)
		}
	}
}

//Close unloads all loaded libs
func (v *VST2) Close() {
	for _, libs := range v.Libs {
		for _, lib := range libs {
			lib.Close()
		}
	}
}

func (v *VST2) loadLibs() filepath.WalkFunc {
	return func(path string, file os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return nil
		}
		if strings.HasSuffix(file.Name(), vst2ext) {
			library, err := vst2.Open(path)
			if err != nil {
				return err
			}
			// add new entry to cache
			dir := filepath.Dir(path)
			if _, ok := v.Libs[dir]; !ok {
				v.Libs[dir] = make(map[string]vst2.Library, 0)
			}
			v.Libs[dir][library.Name] = *library
		}
		return nil
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

func (v VST2) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Scan paths:\n"))
	for _, path := range v.Paths {
		buf.WriteString(fmt.Sprintf("\t%v\n", path))
	}
	buf.WriteString(fmt.Sprintf("Available plugins:\n"))
	buf.WriteString(fmt.Sprintf("%v", v.Libs))
	return buf.String()
}

func (libraries vst2libraries) String() string {
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
func (v *VST2) LoadPlugin(path string, name string) (*vst2.Plugin, error) {
	if len(v.Libs) == 0 {
		return nil, fmt.Errorf("No plugins are found in folders %v", v.Paths)
	}
	libs, ok := v.Libs[path]
	if !ok {
		return nil, fmt.Errorf("Path %v was not scanned", path)
	}
	lib, ok := libs[name]
	if !ok {
		return nil, fmt.Errorf("Library %v was not found at %v", name, path)
	}
	plugin, err := lib.Open()
	if err != nil {
		return nil, fmt.Errorf("Failed to load %v at %v", name, path)
	}

	return plugin, nil
}
