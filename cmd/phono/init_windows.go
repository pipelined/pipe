package main

import "os"

func init() {
	//Windows default VST paths
	defaultVstPaths := []string{
		"C:\\Program Files (x86)\\Steinberg\\VSTPlugins",
		"C:\\Program Files\\Steinberg\\VSTPlugins ",
	}
	VST_PATH := os.Getenv("VST_PATH")
	if len(VST_PATH) > 0 {
		defaultVstPaths = append(defaultVstPaths, VST_PATH)
	}
	vstPaths = defaultVstPaths
}
