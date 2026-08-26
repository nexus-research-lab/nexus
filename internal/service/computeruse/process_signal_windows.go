//go:build windows

package computeruse

import "os"

// Windows desktop hosts replace this fallback with Job Object ownership when
// the native packaging gate lands. Kill is deterministic and never targets a
// process outside the exact supervised handle.
func signalSidecar(process *os.Process) error {
	return process.Kill()
}
