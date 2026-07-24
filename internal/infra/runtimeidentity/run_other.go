//go:build !linux

// Package runtimeidentity 实现 Linux root-owned runtime launcher。
package runtimeidentity

import (
	"fmt"
	"io"
)

// Run 在非 Linux 平台明确拒绝强隔离，避免桌面端误报已启用。
func Run(_ []string, _ []string, _ io.Writer, stderr io.Writer) int {
	_, _ = fmt.Fprintln(stderr, "nexus-runtime-launcher 只支持 Linux")
	return 1
}
