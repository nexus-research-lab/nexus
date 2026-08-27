package main

import (
	"os"

	cli "github.com/nexus-research-lab/nexus/internal/cli"
)

func main() {
	command, err := cli.NewConfiguration(cli.LoadConfigurationConfig())
	if err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
	if err = command.Execute(); err != nil {
		cli.WriteCommandError(os.Stderr, err, cli.RequestedJSON(os.Args[1:]))
		os.Exit(cli.ExitCode(err))
	}
}
