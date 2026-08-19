package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli/agent"
	"github.com/nexus-research-lab/nexus/internal/cli/host"
)

func main() {
	if arguments, ok := agent.RuntimeEntrypointArgs(os.Args[1:]); ok {
		if code := agent.RunRuntime(arguments, os.Stderr); code != 0 {
			os.Exit(code)
		}
		return
	}
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
