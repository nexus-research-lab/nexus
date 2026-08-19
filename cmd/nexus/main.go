package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli/agent"
)

func main() {
	if code := agent.RunRuntime(os.Args[1:], os.Stderr); code != 0 {
		os.Exit(code)
	}
}
