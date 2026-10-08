//go:build windows

package confinedfs

import (
	"os"

	"golang.org/x/sys/windows"
)

// Windows does not expose a link count through os.FileInfo. Keep the
// metadata-only helper for callers that only have an Lstat result; opened
// files use the handle-backed check below.
func hasMultipleHardLinks(os.FileInfo) bool {
	return false
}

func hasMultipleHardLinksFile(file *os.File, _ os.FileInfo) bool {
	if file == nil {
		return true
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		// A protected file whose identity cannot be checked is not safe to
		// expose through the confined filesystem.
		return true
	}
	return info.NumberOfLinks > 1
}
