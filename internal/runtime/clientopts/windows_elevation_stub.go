//go:build !windows

package clientopts

func windowsDesktopSandboxIsElevated() bool { return false }
