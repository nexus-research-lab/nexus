// INPUT: Canonical state root after the Windows shell has reaped the previous backend.
// OUTPUT: Disposable workspace scratch leaves removed before any runtime starts.
// POS: Startup housekeeping under desktop process ownership; never called during live sessions.
package desktopinstance

import (
	"encoding/hex"
	"errors"
	"io/fs"
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// CleanupWindowsScratch requires exclusive desktop ownership and no live runtimes.
// It touches only 64-hex workspace leaves under users/*/runtime/tmp/sandbox.
func CleanupWindowsScratch(stateRoot string) (int, error) {
	root, err := confinedfs.Open(stateRoot)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	users, err := root.OpenRootNoSymlink("users")
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer users.Close()
	owners, err := fs.ReadDir(users.FS(), ".")
	if err != nil {
		return 0, err
	}
	count := 0
	var failures []error
	for _, owner := range owners {
		if !owner.IsDir() {
			continue
		}
		scratch, err := users.OpenRootNoSymlink(owner.Name() + "/runtime/tmp/sandbox")
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			failures = append(failures, err)
			continue
		}
		entries, err := fs.ReadDir(scratch.FS(), ".")
		if err != nil {
			failures = append(failures, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || len(name) != 64 {
				continue
			}
			if _, err := hex.DecodeString(name); err != nil {
				continue
			}
			if err := scratch.RemoveAll(name); err != nil {
				failures = append(failures, err)
			} else {
				count++
			}
		}
		if err := scratch.Close(); err != nil {
			failures = append(failures, err)
		}
	}
	return count, errors.Join(failures...)
}
