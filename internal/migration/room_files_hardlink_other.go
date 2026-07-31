//go:build !darwin && !linux && !windows

package migration

import "os"

func roomFileHasMultipleHardLinks(string, os.FileInfo) (bool, error) {
	return false, nil
}
