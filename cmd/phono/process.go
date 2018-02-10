package main

import (
	"flag"
	"fmt"

	"github.com/dudk/phono/internal/session"
)

type processCommand struct {
	in      string
	out     string
	plugins stringList
	scan    stringList
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
	fs.StringVar(&cmd.in, "in", "", "input audio file to process")
	fs.StringVar(&cmd.out, "out", "", "output file to save processed audio")
	fs.Var(&cmd.plugins, "plugin", "semicolon separated effect names to to use")
}

func (cmd *processCommand) Run() error {
	//vst2 := phono.NewVst2(cmd.scan)
	session := session.New().WithOptions(session.SampleRate(44100.0), session.BufferSize(512))
	fmt.Printf("New session: %v\n", session)
	return nil
}
