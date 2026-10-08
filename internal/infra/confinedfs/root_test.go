package confinedfs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
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

func TestOpenFileNoSymlinkRejectsHardlink(t *testing.T) {
	rootPath := t.TempDir()
	target := filepath.Join(rootPath, "target.jsonl")
	alias := filepath.Join(rootPath, "ledger.jsonl")
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, alias); err != nil {
		t.Skipf("hardlink unavailable: %v", err)
	}
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	file, openErr := root.OpenFileNoSymlink("ledger.jsonl", os.O_RDONLY, 0)
	if runtime.GOOS == "windows" || hasMultipleHardLinks(mustLstat(t, root, "ledger.jsonl")) {
		if !errors.Is(openErr, ErrHardlink) {
			if file != nil {
				file.Close()
			}
			t.Fatalf("OpenFileNoSymlink() error = %v, want ErrHardlink", openErr)
		}
		return
	}
	if openErr != nil {
		t.Fatalf("当前平台不检查硬链接时不应拒绝: %v", openErr)
	}
	file.Close()
}

func TestWriteFileAtomicIfContentRejectsChangedTarget(t *testing.T) {
	root, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err = root.WriteFileAtomic("value.md", []byte("baseline"), 0o660); err != nil {
		t.Fatal(err)
	}
	committed, err := root.WriteFileAtomicIfContent(
		"value.md",
		[]byte("new draft"),
		[]byte("stale baseline"),
		0o660,
	)
	if err != nil {
		t.Fatal(err)
	}
	if committed {
		t.Fatal("过期正文不应替换目标")
	}
	content, err := root.ReadFile("value.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "baseline" {
		t.Fatalf("冲突后 content = %q", content)
	}

	committed, err = root.WriteFileAtomicIfContent(
		"value.md",
		[]byte("new draft"),
		[]byte("baseline"),
		0o660,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("匹配正文应原子替换目标")
	}
}

func TestReadFileSurvivesAtomicReplacement(t *testing.T) {
	rootPath := t.TempDir()
	target := filepath.Join(rootPath, "value.json")
	if err := os.WriteFile(target, []byte(`{"version":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	writerDone := make(chan error, 1)
	go func() {
		for i := 0; i < 128; i++ {
			// 使用生产 writer 的句柄替换；Windows os.Rename 不能代替其 POSIX rename 语义。
			if writeErr := root.WriteFileAtomic("value.json", []byte(`{"version":1}`), 0o600); writeErr != nil {
				writerDone <- writeErr
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
		writerDone <- nil
	}()

	successfulReads := 0
	for {
		select {
		case writerErr := <-writerDone:
			if writerErr != nil {
				t.Fatal(writerErr)
			}
			if successfulReads == 0 {
				t.Fatal("no read completed while the writer was running")
			}
			if body, err := root.ReadFile("value.json"); err != nil || !bytes.Equal(body, []byte(`{"version":1}`)) {
				t.Fatalf("ReadFile() after atomic replacement: body=%q, err=%v", body, err)
			}
			return
		default:
			body, readErr := root.ReadFile("value.json")
			// 持续替换可能耗尽有界身份重试；保守拒绝属于合同，不能要求 reader 放宽校验。
			if errors.Is(readErr, ErrChanged) {
				continue
			}
			if readErr != nil {
				<-writerDone
				t.Fatalf("ReadFile() during atomic replacement: %v", readErr)
			}
			if !bytes.Equal(body, []byte(`{"version":0}`)) && !bytes.Equal(body, []byte(`{"version":1}`)) {
				<-writerDone
				t.Fatalf("ReadFile() returned a partial document: %q", body)
			}
			successfulReads++
		}
	}
}

func mustLstat(t *testing.T, root *Root, name string) os.FileInfo {
	t.Helper()
	info, err := root.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
