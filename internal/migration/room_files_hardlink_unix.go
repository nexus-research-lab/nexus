//go:build darwin || linux

package migration

import (
	"os"
	"syscall"
)

func roomFileHasMultipleHardLinks(_ string, info os.FileInfo) (bool, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink > 1, nil
}
