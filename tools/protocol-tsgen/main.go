package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	outputPath := filepath.Join(root, "web", "src", "types", "generated", "protocol.ts")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile(outputPath, []byte(protocol.TypeScriptDefinitions()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(outputPath)
}

func findRepoRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if looksLikeRepoRoot(current) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("repo root not found from %s", current)
		}
		current = parent
	}
}

func looksLikeRepoRoot(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "web", "package.json")); err != nil {
		return false
	}
	return true
}
