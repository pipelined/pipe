package main

import (
	"flag"
	"fmt"

	"github.com/dudk/phono"
)

type listCommand struct {
	scan stringList
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
	vst2 := phono.NewVst2(cmd.scan)
	fmt.Printf("Scan paths:\n %v\n", vst2.Paths)
	fmt.Printf("Available plugins:\n %v\n", vst2.Libs)
	return nil
}
