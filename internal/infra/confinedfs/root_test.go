package confinedfs

import (
	"errors"
	"os"
	"path/filepath"
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

func TestOpenRootNoSymlinkRejectsLinkWithinRoot(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "owners", "user-b", "rooms"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join("..", "user-b"),
		filepath.Join(rootPath, "owners", "user-a"),
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if child, openErr := root.OpenRootNoSymlink("owners/user-a/rooms"); !errors.Is(openErr, ErrSymlink) {
		if child != nil {
			child.Close()
		}
		t.Fatalf("OpenRootNoSymlink() error = %v, want ErrSymlink", openErr)
	}
}

func TestOpenOrCreateRootNoSymlinkCreatesStableTree(t *testing.T) {
	root, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	child, err := root.OpenOrCreateRootNoSymlink("users/user-a/state/rooms", 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	if info, statErr := child.Stat("."); statErr != nil || !info.IsDir() {
		t.Fatalf("创建后的目录不可用: info=%v err=%v", info, statErr)
	}
}

func TestOpenFileNoSymlinkRejectsLinkWithinRoot(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, "target.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.jsonl", filepath.Join(rootPath, "ledger.jsonl")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if file, openErr := root.OpenFileNoSymlink("ledger.jsonl", os.O_RDONLY, 0); !errors.Is(openErr, ErrSymlink) {
		if file != nil {
			file.Close()
		}
		t.Fatalf("OpenFileNoSymlink() error = %v, want ErrSymlink", openErr)
	}
}

func TestOpenFileNoSymlinkRejectsIntermediateLinkWithinRoot(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootPath, "target"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(rootPath, "target", "ledger.jsonl"),
		[]byte("{}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(rootPath, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if file, openErr := root.OpenFileNoSymlink("linked/ledger.jsonl", os.O_RDONLY, 0); !errors.Is(openErr, ErrSymlink) {
		if file != nil {
			file.Close()
		}
		t.Fatalf("OpenFileNoSymlink() error = %v, want ErrSymlink", openErr)
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
	if hasMultipleHardLinks(mustLstat(t, root, "ledger.jsonl")) {
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
			temporary := filepath.Join(rootPath, "value.tmp")
			if writeErr := os.WriteFile(temporary, []byte(`{"version":1}`), 0o600); writeErr != nil {
				writerDone <- writeErr
				return
			}
			if renameErr := os.Rename(temporary, target); renameErr != nil {
				writerDone <- renameErr
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
		writerDone <- nil
	}()

	for {
		select {
		case writerErr := <-writerDone:
			if writerErr != nil {
				t.Fatal(writerErr)
			}
			return
		default:
			if _, readErr := root.ReadFile("value.json"); readErr != nil {
				<-writerDone
				t.Fatalf("ReadFile() during atomic replacement: %v", readErr)
			}
		}
	}
}

func TestCopyTreeFromRejectsSourceSymlink(t *testing.T) {
	sourcePath := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(sourcePath, "linked.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	source, err := Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()

	if err = target.CopyTreeFrom(source); !errors.Is(err, ErrSymlink) {
		t.Fatalf("CopyTreeFrom() error = %v, want ErrSymlink", err)
	}
	if _, err = target.Lstat("linked.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("目标不应包含符号链接内容: %v", err)
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
