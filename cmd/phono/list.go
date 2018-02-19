package main

import (
	"flag"
	"fmt"

	"github.com/dudk/phono/cache"
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
	cache := cache.NewVST2(cmd.scan...)
	fmt.Print(cache)
	return nil
}

func (cmd *listCommand) Validate() error {
	return nil
}
