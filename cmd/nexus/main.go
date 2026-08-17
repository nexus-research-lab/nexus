package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli"
)

func main() {
	command, err := cli.NewRuntime(cli.LoadRuntimeConfig())
	if err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
	if err = command.Execute(); err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
}
