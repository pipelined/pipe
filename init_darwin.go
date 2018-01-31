package main

func init() {
	//OS X default VST paths
	defaultScanPaths := []string{
		"~/Library/Audio/Plug-Ins/VST",
		"/Library/Audio/Plug-Ins/VST",
	}
	scanPaths = defaultScanPaths
}
