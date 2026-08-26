//go:build !windows

// INPUT: a fully fsynced temporary file and its destination in the same directory.
// OUTPUT: one atomic current-package marker replacement on Unix hosts.
// POS: package activation commit primitive; callers own content validation and directory layout.
package computeruse

import "os"

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}
