//go:build windows

package runtime

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// sandboxProcessIdentity uses the native process creation timestamp so a
// recycled PID cannot make a stale scratch marker look owned by the current
// process. Access failures are unknown and therefore retained by recovery.
func sandboxProcessIdentity(pid int) (startTimeUnixNano int64, alive, known bool) {
	if pid <= 0 {
		return 0, false, false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_FOUND) {
			return 0, false, true
		}
		return 0, false, false
	}
	defer windows.CloseHandle(handle)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return 0, false, true
		}
		return 0, false, false
	}
	startTimeUnixNano = creation.Nanoseconds()
	if startTimeUnixNano <= 0 {
		return 0, true, false
	}
	return startTimeUnixNano, true, true
}

func sandboxProcessAlive(pid int) (alive, known bool) {
	_, alive, known = sandboxProcessIdentity(pid)
	return alive, known
}

func currentProcessStartTimeUnixNano() (int64, error) {
	return processStartTimeUnixNano(os.Getpid())
}

func processStartTimeUnixNano(pid int) (int64, error) {
	start, alive, known := sandboxProcessIdentity(pid)
	if !known {
		return 0, errors.New("windows process creation time is unavailable")
	}
	if !alive {
		return 0, errors.New("windows process is not alive")
	}
	if start <= 0 {
		return 0, errors.New("windows process creation time is invalid")
	}
	return start, nil
}
