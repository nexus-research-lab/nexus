//go:build darwin || linux

package confinedfs

import (
	"os"
	"syscall"
)

func hasMultipleHardLinks(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink > 1
}
