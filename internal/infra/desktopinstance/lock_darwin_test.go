//go:build darwin

// INPUT: 独立状态目录、真实子进程和路径替换反例。
// OUTPUT: 第二 sidecar 拒绝、持有者退出后释放、CLOEXEC 及 inode 变更拒绝。
// POS: 内核实例锁验收，不声明旧版本宿主或 runtime 后代已经退出。
package desktopinstance

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDesktopInstanceExclusiveAndStable(t *testing.T) {
	root := t.TempDir()
	guard, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	for range 2 {
		if err := guard.Verify(); err != nil {
			t.Fatal(err)
		}
		if other, err := Acquire(root); !errors.Is(err, ErrInUse) {
			if other != nil {
				other.Close()
			}
			t.Fatalf("second owner=%v", err)
		}
	}
	flags, err := unix.FcntlInt(guard.file.Fd(), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("lock inherited: %d %v", flags, err)
	}
	path := filepath.Join(root, "app", "sidecar.lock")
	old, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	if err := guard.Verify(); err == nil {
		t.Fatal("closed guard verified")
	}
	next, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	current, err := os.Stat(path)
	if err != nil || !os.SameFile(old, current) {
		t.Fatal("persistent lock inode replaced", err)
	}
}

func TestDesktopInstanceRejectsLinkAndReplacement(t *testing.T) {
	for _, mode := range []string{"symlink", "hardlink", "file_replace", "directory_replace"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			app := filepath.Join(root, "app")
			if err := os.Mkdir(app, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(app, "sidecar.lock")
			target := filepath.Join(t.TempDir(), "keep")
			if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "symlink" || mode == "hardlink" {
				link := os.Symlink
				if mode == "hardlink" {
					link = os.Link
				}
				if err := link(target, path); err != nil {
					t.Fatal(err)
				}
				if guard, err := Acquire(root); err == nil {
					guard.Close()
					t.Fatal("link accepted")
				}
			} else {
				guard, err := Acquire(root)
				if err != nil {
					t.Fatal(err)
				}
				defer guard.Close()
				if mode == "file_replace" {
					if err := os.Rename(path, path+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Rename(app, app+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(app, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := guard.Verify(); err == nil {
					t.Fatal("replaced identity accepted")
				}
			}
			content, err := os.ReadFile(target)
			if err != nil || string(content) != "untouched" {
				t.Fatal("outside content changed", err)
			}
		})
	}
}

func TestDesktopInstanceReleasedByProcessExit(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDesktopInstanceHolder$")
	cmd.Env = append(os.Environ(), "NEXUS_INSTANCE_TEST_ROOT="+root)
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); _ = cmd.Wait() }()
	ready := bufio.NewScanner(output)
	if !ready.Scan() || ready.Text() != "ready" {
		t.Fatalf("child not ready: %v", ready.Err())
	}
	if guard, err := Acquire(root); !errors.Is(err, ErrInUse) {
		if guard != nil {
			guard.Close()
		}
		t.Fatalf("live process did not hold lock: %v", err)
	}
	input.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	guard, err := Acquire(root)
	if err != nil {
		t.Fatalf("exited holder retained lock: %v", err)
	}
	guard.Close()
}
func TestDesktopInstanceHolder(t *testing.T) {
	root := os.Getenv("NEXUS_INSTANCE_TEST_ROOT")
	if root == "" {
		return
	}
	guard, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.Verify(); err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString("ready\n")
	var b [1]byte
	os.Stdin.Read(b[:])
	// 不调用 Close；由真实进程退出释放锁，不通过 PID 或时间推断。
	os.Exit(0)
}

func TestDesktopRecoveryOwnershipPinsLockThroughCallback(t *testing.T) {
	root := t.TempDir()
	guard, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	entered, release := make(chan string, 1), make(chan struct{})
	releaseCallback := sync.OnceFunc(func() { close(release) })
	defer releaseCallback()
	finished := make(chan error, 1)
	go func() {
		finished <- guard.WithOwnership(func(appRoot string) error { entered <- appRoot; <-release; return nil })
	}()
	appRoot := <-entered
	expected, err := os.Stat(filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.Stat(appRoot)
	if err != nil || !os.SameFile(expected, actual) {
		t.Fatalf("owned root=%q err=%v", appRoot, err)
	}
	closeStarted, closed := make(chan struct{}), make(chan error, 1)
	go func() { close(closeStarted); closed <- guard.Close() }()
	<-closeStarted
	select {
	case err := <-closed:
		releaseCallback()
		t.Fatalf("lock closed during recovery: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if other, err := Acquire(root); !errors.Is(err, ErrInUse) {
		if other != nil {
			other.Close()
		}
		releaseCallback()
		t.Fatalf("recovery lost exclusive lock: %v", err)
	}
	releaseCallback()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	invoked := false
	if err := guard.WithOwnership(func(string) error { invoked = true; return nil }); err == nil || invoked {
		t.Fatal("closed guard admitted recovery")
	}
	next, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
}
