package main

import (
	"os"

	cli "github.com/nexus-research-lab/nexus/internal/cli"
)

func main() {
	command, err := cli.New(cli.LoadConfig())
	if err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
	if err = command.Execute(); err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
}
