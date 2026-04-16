// =====================================================
// @File   ：main.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package main

import (
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	"os"
	"path/filepath"
)

func main() {
	outputPath := filepath.Join("web", "src", "types", "generated", "protocol.ts")
	if err := os.WriteFile(outputPath, []byte(protocol.TypeScriptDefinitions()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(outputPath)
}
