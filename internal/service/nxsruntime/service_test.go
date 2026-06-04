package nxsruntime

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStatusUsesEnvCommandPath(t *testing.T) {
	runtimePath := writeExecutableForTest(t, t.TempDir())
	service := NewService()
	service.getenv = func(key string) string {
		if key == commandPathEnvName {
			return runtimePath
		}
		return ""
	}

	status := service.Status()
	if !status.Available || status.Path != runtimePath || status.Source != "env" {
		t.Fatalf("Status() = %+v, want env runtime", status)
	}
}

func TestStatusRejectsBrokenEnvCommandPath(t *testing.T) {
	brokenPath := filepath.Join(t.TempDir(), executableName())
	service := NewService()
	service.getenv = func(key string) string {
		if key == commandPathEnvName {
			return brokenPath
		}
		return ""
	}

	status := service.Status()
	if status.Available || status.CanDownload || status.Source != "env" {
		t.Fatalf("Status() = %+v, want broken env without download", status)
	}
}

func TestStatusUsesAppRootRuntime(t *testing.T) {
	root := t.TempDir()
	runtimePath := writeExecutableForTest(t, filepath.Join(root, "bin"))
	service := NewService()
	service.getenv = func(string) string { return "" }
	service.appRoot = func() string { return root }

	status := service.Status()
	if !status.Available || status.Path != runtimePath || status.Source != "app_root" {
		t.Fatalf("Status() = %+v, want app root runtime", status)
	}
}

func TestStatusUsesCachedRuntime(t *testing.T) {
	cacheRoot := t.TempDir()
	runtimePath := writeExecutableForTest(t, filepath.Join(cacheRoot, "0.1.1", runtime.GOOS+"-"+runtime.GOARCH, "digest"))
	service := NewService()
	service.getenv = func(key string) string {
		if key == cacheDirEnvName {
			return cacheRoot
		}
		return ""
	}
	service.appRoot = func() string { return t.TempDir() }

	status := service.Status()
	if !status.Available || status.Path != runtimePath || status.Source != "cache" {
		t.Fatalf("Status() = %+v, want cached runtime", status)
	}
}

func TestDownloadUsesBridgeResolver(t *testing.T) {
	runtimePath := writeExecutableForTest(t, t.TempDir())
	service := NewService()
	service.getenv = func(string) string { return "" }
	service.appRoot = func() string { return t.TempDir() }
	service.userCache = func() (string, error) {
		return "", errors.New("no cache")
	}
	service.runtimePath = func() (string, error) {
		return runtimePath, nil
	}

	status, err := service.Download()
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !status.Available || status.Path != runtimePath || status.Source != "cache" {
		t.Fatalf("Download() = %+v, want downloaded runtime", status)
	}
}

func writeExecutableForTest(t *testing.T, directory string) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	path := filepath.Join(directory, executableName())
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("写入 runtime 失败: %v", err)
	}
	return path
}
