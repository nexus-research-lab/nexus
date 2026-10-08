//go:build windows

// INPUT: 持久资源 marker 的 PID 与 Windows 内核进程对象。
// OUTPUT: 创建时间、真实存活状态以及能否确认；访问失败保留未知。
// POS: 宿主崩溃后的显式资源对账，不用句柄仍存在或退出码 259 推断存活。
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
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_FOUND) {
			return 0, false, true
		}
		return 0, false, false
	}
	defer windows.CloseHandle(handle)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, false, false
	}
	startTimeUnixNano = creation.Nanoseconds()
	// 进程终止后只要仍有句柄，PID 和 GetProcessTimes 仍可用；只有内核信号
	// 能区分该状态。GetExitCodeProcess 的 STILL_ACTIVE 也可能是合法退出码。
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return startTimeUnixNano, false, false
	}
	if status == windows.WAIT_OBJECT_0 {
		return startTimeUnixNano, false, true
	}
	if status != uint32(windows.WAIT_TIMEOUT) {
		return startTimeUnixNano, false, false
	}
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
