package desktopinstance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScratchCleanupOnlyRemovesDisposableWorkspaceLeaves(t *testing.T) {
	state := t.TempDir()
	base := filepath.Join(state, "users", "owner", "runtime", "tmp", "sandbox")
	leaf := filepath.Join(base, strings.Repeat("a", 64))
	keep := filepath.Join(base, "user-data")
	for _, dir := range []string{leaf, keep} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	n, err := CleanupWindowsScratch(state)
	if err != nil || n != 1 {
		t.Fatalf("removed=%d err=%v", n, err)
	}
	if _, err := os.Stat(leaf); !os.IsNotExist(err) {
		t.Fatal("scratch retained")
	}
	if _, err := os.Stat(filepath.Join(keep, "file")); err != nil {
		t.Fatal("non-scratch data removed")
	}
	if n, err := CleanupWindowsScratch(state); err != nil || n != 0 {
		t.Fatalf("retry=%d err=%v", n, err)
	}
}

func TestScratchCleanupRefusesLinkedParent(t *testing.T) {
	state, outside := t.TempDir(), t.TempDir()
	owner := filepath.Join(state, "users", "owner", "runtime")
	if err := os.MkdirAll(owner, 0700); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(outside, "sandbox", strings.Repeat("b", 64))
	if err := os.MkdirAll(leaf, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(owner, "tmp")); err != nil {
		t.Skip(err)
	}
	if _, err := CleanupWindowsScratch(state); err == nil {
		t.Fatal("linked parent accepted")
	}
	if _, err := os.Stat(leaf); err != nil {
		t.Fatal("outside data removed")
	}
}
