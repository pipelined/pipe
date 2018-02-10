package main

import (
	"flag"
)

type processCommand struct {
	scan scanPaths
}

//Implement phono.command interface
func (cmd *processCommand) Name() string {
	return "process"
}

func (cmd *processCommand) Help() string {
	return "Process audio with listed effects"
}

func (cmd *processCommand) Register(fs *flag.FlagSet) {
	fs.Var(&cmd.scan, "scan", "semicolon separated paths to scan for effects")
}

func (cmd *processCommand) Run() error {
	//vst2 := phono.NewVst2(cmd.scan)
	return nil
}
