//go:build windows

package clientopts

import "golang.org/x/sys/windows"

func windowsDesktopSandboxIsElevated() bool {
	// The current-process token is a pseudo handle and must not be closed.
	return windows.GetCurrentProcessToken().IsElevated()
}
