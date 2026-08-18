package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli"
)

func main() {
	if arguments, ok := cli.RuntimeEntrypointArgs(os.Args[1:]); ok {
		if code := cli.RunRuntime(arguments, os.Stderr); code != 0 {
			os.Exit(code)
		}
		return
	}
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
