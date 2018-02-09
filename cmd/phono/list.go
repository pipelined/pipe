package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/dudk/phono"
)

type listCommand struct {
	scan scanPaths
}

type scanPaths []string

//Implement the flag.Value interface
func (s *scanPaths) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *scanPaths) Set(value string) error {
	*s = strings.Split(value, ";")
	return nil
}

//Implement phono.command interface
func (cmd *listCommand) Name() string {
	return "list"
}

func (cmd *listCommand) Help() string {
	return "Show the list of available plugins"
}

func (cmd *listCommand) Register(fs *flag.FlagSet) {
	fs.Var(&cmd.scan, "scan", "semicolon separated paths to scan for effects")
}

func (cmd *listCommand) Run() error {
	scanPaths := append(getVstScanPaths(), cmd.scan...)
	vst2 := phono.NewVst2(scanPaths)
	fmt.Printf("Scan paths:\n %v\n", scanPaths)
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
