package confinedfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRootRejectsTraversalAndAbsolutePaths(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	for _, name := range []string{"../outside", "/tmp/outside", "nested/../../outside", "C:/outside", `C:\outside`} {
		if _, err := root.Stat(name); !errors.Is(err, ErrParentTraversal) && !errors.Is(err, ErrAbsolutePath) {
			t.Fatalf("Stat(%q) error = %v, want confined path error", name, err)
		}
	}
}

func TestOpenRejectsSymlinkRoot(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if root, err := Open(link); err == nil {
		_ = root.Close()
		t.Fatal("符号链接不能作为 confined root")
	}
}

func TestRootBlocksSymlinkEscape(t *testing.T) {
	rootPath := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(rootPath, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if _, err = root.ReadFile("escape"); err == nil {
		t.Fatal("ReadFile followed symlink outside confined root")
	}
}

func TestRootBlocksIntermediateSymlinkWrite(t *testing.T) {
	rootPath := t.TempDir()
	outsidePath := t.TempDir()
	if err := os.Symlink(outsidePath, filepath.Join(rootPath, "nested")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err = root.WriteFileAtomic("nested/value.txt", []byte("escaped"), 0o600); err == nil {
		t.Fatal("WriteFileAtomic followed intermediate symlink outside confined root")
	}
	if _, err = os.Stat(filepath.Join(outsidePath, "value.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file unexpectedly created: %v", err)
	}
}

func TestWriteFileAtomicAndRenameRemainWithinRoot(t *testing.T) {
	root, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err = root.WriteFileAtomic("nested/value.json", []byte(`{"ok":true}`), 0o660); err != nil {
		t.Fatal(err)
	}
	content, err := root.ReadFile("nested/value.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != `{"ok":true}` {
		t.Fatalf("content = %q", content)
	}
	if err = root.Rename("nested/value.json", "nested/renamed.json"); err != nil {
		t.Fatal(err)
	}
	if _, err = root.Stat("nested/value.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old path still exists: %v", err)
	}
}
