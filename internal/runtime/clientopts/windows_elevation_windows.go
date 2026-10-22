//go:build windows

package clientopts

import "golang.org/x/sys/windows"

// windowsDesktopSandboxElevated rejects restricted MXC sessions launched with
// an elevated token. Native MXC isolation is validated for ordinary desktop
// user sessions; an administrator token must fail closed instead of silently
// weakening the resource boundary.
func windowsDesktopSandboxElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
