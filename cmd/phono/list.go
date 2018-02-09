package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/dudk/phono"
)

type listCommand struct {
	scan string
}

func (cmd *listCommand) Name() string {
	return "list"
}

func (cmd *listCommand) Help() string {
	return "Show the list of available plugins"
}

func (cmd *listCommand) Register(fs *flag.FlagSet) {
	fs.StringVar(&cmd.scan, "scan", "", "semicolon-separated paths to scan for effects")
}

func (cmd *listCommand) Run() error {
	vst2 := phono.NewVst2(getVstScanPaths())
	fmt.Printf("Available plugins:\n %v\n", vst2)
	return nil
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
