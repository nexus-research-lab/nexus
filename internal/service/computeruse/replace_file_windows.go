//go:build windows

// INPUT: a fully flushed temporary file and its destination in the same directory.
// OUTPUT: one replace-existing, write-through current-package marker move on Windows.
// POS: package activation commit primitive; callers own content validation and directory layout.
package computeruse

import "golang.org/x/sys/windows"

func replaceFile(source, target string) error {
	sourcePath, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPath, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		sourcePath,
		targetPath,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
