//go:build !windows

package clientopts

func windowsDesktopSandboxElevated() bool { return false }
