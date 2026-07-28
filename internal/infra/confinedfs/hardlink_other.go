//go:build !darwin && !linux

package confinedfs

import "os"

func hasMultipleHardLinks(os.FileInfo) bool {
	return false
}
