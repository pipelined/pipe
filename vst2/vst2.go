package vst2

import vst2 "github.com/dudk/vst2"

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
