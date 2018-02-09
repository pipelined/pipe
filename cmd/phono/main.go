package main

import (
	"flag"
	"fmt"
	"os"
)

type config struct {
	args []string
}

type command interface {
	Name() string
	Help() string
	Run() error
	Register(*flag.FlagSet)
}

func (config *config) run() int {
	cmdName, args := parseArgs(config.args)
	if cmdName == "" {
		printUsage()
		return errorExitCode
	}

	for _, cmd := range commands {
		if cmd.Name() == cmdName {
			flags := flag.NewFlagSet(cmdName, flag.ExitOnError)
			cmd.Register(flags)
			if err := flags.Parse(args); err != nil {
				flags.PrintDefaults()
				return errorExitCode
			}
			if err := cmd.Run(); err != nil {
				fmt.Printf("Command failed: %v", err)
				return errorExitCode
			}
		}
	}

	return successExitCode
}

var (
	successExitCode = 0
	errorExitCode   = 1
	commands        []command
)

func main() {
	commands = []command{&listCommand{}}
	c := config{
		args: os.Args,
	}
	os.Exit(c.run())
}

func parseArgs(args []string) (string, []string) {
	if len(args) < 2 {
		return "", nil
	}
	return args[1], args[2:]
}

func printUsage() {
	fmt.Println("Phono is a CLI VST host")
	fmt.Println()
	fmt.Println("Usage: phono <command>")
	fmt.Println()
	fmt.Println("Commands:")
	for _, cmd := range commands {
		fmt.Printf("\t%s\t%s\n", cmd.Name(), cmd.Help())
	}
}
