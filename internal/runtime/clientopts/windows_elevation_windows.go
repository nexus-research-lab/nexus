//go:build windows

package clientopts

import "golang.org/x/sys/windows"

func windowsDesktopSandboxIsElevated() bool {
	token := windows.GetCurrentProcessToken()
	defer token.Close()
	return token.IsElevated()
}
