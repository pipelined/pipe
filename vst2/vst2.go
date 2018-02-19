package vst2

import (
	"os"
	"runtime"

	"github.com/dudk/vst2"
)

//Open loads a library
func Open(path string) (*Library, error) {
	lib, err := vst2.Open(path)
	if err != nil {
		return nil, err
	}
	return &Library{
		Library: lib,
	}, nil
}

//Plugin is a wrapper for vst2.Plugin
type Plugin struct {
	*vst2.Plugin
}

//Library is a wrapper over vst2 sdk type
type Library struct {
	*vst2.Library
}

//Open loads vst2 plugin
func (l Library) Open() (*Plugin, error) {
	plugin, err := l.Library.Open()
	if err != nil {
		return nil, err
	}
	return &Plugin{
		Plugin: plugin,
	}, nil
}

// DefaultScanPaths returns a slice of default vst2 locations
func DefaultScanPaths() (paths []string) {
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

// FileExtension returns default vst2 extension
func FileExtension() string {
	switch os := runtime.GOOS; os {
	case "darwin":
		return ".vst"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}
