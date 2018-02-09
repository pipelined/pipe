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
	cmdName, help := parseArgs(config.args)
	if cmdName == "" {
		if help {
			printUsage()
		}
		return errorExitCode
	}

	for _, cmd := range commands {
		if cmd.Name() == cmdName {
			flags := flag.NewFlagSet(cmdName, flag.ExitOnError)
			cmd.Register(flags)
			flags.Parse(config.args[2:])
			if help {
				//TODO: add full help for command
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
	commands        = []command{
		&listCommand{},
	}
)

func main() {
	c := config{
		args: os.Args,
	}
	os.Exit(c.run())
}

func parseArgs(args []string) (string, bool) {
	switch len(args) {
	case 0, 1:
		return "", false
	case 2:
		if args[1] == "help" {
			return "", true
		}
		return args[1], false
	default:
		if args[1] == "help" {
			return args[2], true
		}
		return args[1], false
	}
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
	fmt.Printf("Use \"phono help [command]\" for more information about a command.\n")
}
