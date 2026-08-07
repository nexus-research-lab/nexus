// 本文件验证 workspace 遍历只跳过不可读子树，不掩盖根目录或其他错误。
package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleWorkspaceWalkErrorSkipsUnreadableDirectory(t *testing.T) {
	directoryInfo, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	got := handleWorkspaceWalkError(
		"memory",
		fs.FileInfoToDirEntry(directoryInfo),
		&fs.PathError{Op: "open", Path: "memory", Err: os.ErrPermission},
	)
	if !errors.Is(got, fs.SkipDir) {
		t.Fatalf("handleWorkspaceWalkError() = %v, want SkipDir", got)
	}
}

func TestHandleWorkspaceWalkErrorSkipsUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.md")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	got := handleWorkspaceWalkError(
		"private.md",
		fs.FileInfoToDirEntry(fileInfo),
		&fs.PathError{Op: "stat", Path: "private.md", Err: os.ErrPermission},
	)
	if got != nil {
		t.Fatalf("handleWorkspaceWalkError() = %v, want nil", got)
	}
}

func TestHandleWorkspaceWalkErrorPreservesBoundaryErrors(t *testing.T) {
	permissionErr := &fs.PathError{Op: "open", Path: ".", Err: os.ErrPermission}
	if got := handleWorkspaceWalkError(".", nil, permissionErr); !errors.Is(got, os.ErrPermission) {
		t.Fatalf("root error = %v, want permission error", got)
	}

	changedErr := errors.New("workspace changed")
	if got := handleWorkspaceWalkError("memory", nil, changedErr); !errors.Is(got, changedErr) {
		t.Fatalf("non-permission error = %v, want original error", got)
	}
}
