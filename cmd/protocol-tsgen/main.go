package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func main() {
	outputPath := filepath.Join("web", "src", "types", "generated", "protocol.ts")
	if err := os.WriteFile(outputPath, []byte(protocol.TypeScriptDefinitions()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(outputPath)
}
