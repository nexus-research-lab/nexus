package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli"
	"github.com/nexus-research-lab/nexus/internal/config"
)

func main() {
	command, err := cli.New(config.Load())
	if err != nil {
		os.Exit(1)
	}
	if err = command.Execute(); err != nil {
		os.Exit(1)
	}
}
