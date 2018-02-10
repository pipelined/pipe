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
	fs.StringVar(&cmd.in, "in", "", "input audio file to process (required)")
	fs.StringVar(&cmd.out, "out", "", "output file to save processed audio (required)")
	fs.Var(&cmd.plugins, "plugin", "semicolon separated effect names to to use (required)")
}

func (cmd *processCommand) Run() error {
	err := cmd.Validate()
	if err != nil {
		return err
	}
	//vst2 := phono.NewVst2(cmd.scan)
	session := session.New().WithOptions(session.SampleRate(44100.0), session.BufferSize(512))
	fmt.Printf("New session: %v\n", session)
	fmt.Printf("Process command: %v\n", cmd)
	return nil
}

func (cmd *processCommand) Validate() error {
	var message string
	if cmd.in == "" {
		message = message + fmt.Sprintf("Missing -in required flag\n")
	}
	if cmd.out == "" {
		message = message + fmt.Sprintf("Missing -out required flag\n")
	}
	if cmd.plugins == nil || len(cmd.plugins) == 0 {
		message = message + fmt.Sprintf("Missing -plugin required flag\n")
	}
	if message != "" {
		return fmt.Errorf(message)
	}
	return nil
}
