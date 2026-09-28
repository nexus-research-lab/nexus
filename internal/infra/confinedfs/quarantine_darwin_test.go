//go:build darwin

package confinedfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQuarantineDirectoryKeepsPinnedRootsAndRefusesReplacement(t *testing.T) {
	base := t.TempDir()
	sourcePath := filepath.Join(base, "source")
	destPath := filepath.Join(base, "dest")
	for _, p := range []string{sourcePath, destPath, filepath.Join(sourcePath, "leaf")} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	source, err := Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	dest, err := Open(destPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()
	identity, err := source.Lstat("leaf")
	if err != nil {
		t.Fatal(err)
	}
	if err := dest.Mkdir("slot", 0700); err != nil {
		t.Fatal(err)
	}
	if err := source.QuarantineDirectory("leaf", dest, "slot", identity); err == nil {
		t.Fatal("overwrote occupied recovery slot")
	}
	if _, err := source.Stat("leaf"); err != nil {
		t.Fatal("source moved on conflict")
	}
	if err := dest.Remove("slot"); err != nil {
		t.Fatal(err)
	}
	// Published path replacement must not redirect either pinned root.
	if err := os.Rename(destPath, destPath+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := source.QuarantineDirectory("leaf", dest, "slot", identity); err != nil {
		t.Fatal(err)
	}
	got, err := dest.Stat("slot")
	if err != nil || !os.SameFile(identity, got) {
		t.Fatalf("wrong moved identity: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destPath, "slot")); !os.IsNotExist(err) {
		t.Fatal("followed replaced destination path")
	}
	if err := dest.SyncDirectory(); err != nil {
		t.Fatal(err)
	}
}

func TestQuarantineDirectoryRejectsChangedSourceAndTraversal(t *testing.T) {
	source, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	dest, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()
	if err := source.Mkdir("leaf", 0700); err != nil {
		t.Fatal(err)
	}
	identity, err := source.Stat("leaf")
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Rename("leaf", "original"); err != nil {
		t.Fatal(err)
	}
	if err := source.Mkdir("leaf", 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"leaf", "../original", "/original", "."} {
		if err := source.QuarantineDirectory(name, dest, "slot", identity); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err := source.Stat("leaf"); err != nil {
		t.Fatal("removed replacement")
	}
	if _, err := dest.Stat("slot"); !os.IsNotExist(err) {
		t.Fatal("moved unverified source")
	}
}
