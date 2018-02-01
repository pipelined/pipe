package main

func init() {
	//OS X default VST paths
	defaultVstPaths := []string{
		"~/Library/Audio/Plug-Ins/VST",
		"/Library/Audio/Plug-Ins/VST",
	}
	vstPaths = defaultVstPaths
}
