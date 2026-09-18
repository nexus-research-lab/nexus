//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package runtime

import (
	"errors"
	"syscall"
)

// sandboxProcessAlive uses the platform's non-destructive signal probe. A
// permission error still proves that a process exists; any other unexpected
// error is unknown and therefore retained by the sweep.
func sandboxProcessAlive(pid int) (alive, known bool) {
	if pid <= 0 {
		return false, false
	}
	err := syscall.Kill(pid, 0)
	switch {
	case err == nil:
		return true, true
	case errors.Is(err, syscall.ESRCH):
		return false, true
	case errors.Is(err, syscall.EPERM):
		return true, true
	default:
		return false, false
	}
}
