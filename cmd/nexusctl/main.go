package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli/host"
)

func main() {
	command, err := host.New(host.LoadConfig())
	if err != nil {
		host.WriteCommandError(os.Stderr, err, host.RequestedJSON(os.Args[1:]))
		os.Exit(host.ExitCode(err))
	}
	if err = command.Execute(); err != nil {
		host.WriteCommandError(os.Stderr, err, host.RequestedJSON(os.Args[1:]))
		os.Exit(host.ExitCode(err))
	}
}
