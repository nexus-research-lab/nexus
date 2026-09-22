//go:build !darwin && !linux && !windows

package confinedfs

import "os"

func hasMultipleHardLinks(os.FileInfo) bool {
	return false
}

func hasMultipleHardLinksFile(_ *os.File, _ os.FileInfo) bool {
	return false
}
