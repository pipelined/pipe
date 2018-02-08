package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/dudk/phono"
)

var vstPaths = getVstScanPaths()

var vst2 phono.Vst2

func main() {

	fmt.Printf("%v\n", vstPaths)
	vst2 = phono.NewVst2(vstPaths)

	fmt.Println(vst2)
}

func getVstScanPaths() (paths []string) {
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
