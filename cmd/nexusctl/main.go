// =====================================================
// @File   ：main.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

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
