package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/cli"
)

func main() {
	if code := cli.RunRuntime(os.Args[1:], os.Stderr); code != 0 {
		os.Exit(code)
	}
}
