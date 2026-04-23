package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli"
	"github.com/nexus-research-lab/nexus/internal/config"
)

func main() {
	command, err := cli.New(config.Load())
	if err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
	if err = command.Execute(); err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
}
