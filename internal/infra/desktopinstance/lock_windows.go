//go:build windows

// INPUT: 宿主固定的完整状态根。
// OUTPUT: canonical app 目录中的内核独占锁，或明确已被另一 sidecar 占用。
// POS: sidecar 实例所有权，不充当 runtime 后代退出或旧版宿主退出证据。
package desktopinstance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"golang.org/x/sys/windows"
)

var ErrInUse = errors.New("desktop state root is already owned by another sidecar")

// Guard 持有不继承的 Windows 文件句柄，不向工具/helper 传递实例锁。
// 文件从不删除，锁字节固定为首字节；关闭句柄或宿主退出由内核释放。
type Guard struct {
	mu       sync.Mutex
	root     *confinedfs.Root
	file     *os.File
	closeErr error
}

func Acquire(stateRoot string) (*Guard, error) {
	if !filepath.IsAbs(stateRoot) || filepath.Clean(stateRoot) == string(filepath.Separator) {
		return nil, errors.New("desktop state root is empty")
	}
	if err := os.MkdirAll(stateRoot, 0700); err != nil {
		return nil, err
	}
	state, err := confinedfs.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	defer state.Close()
	root, err := state.OpenOrCreateRootNoSymlink("app", 0700)
	if err != nil {
		return nil, err
	}
	file, err := root.OpenFileNoSymlink("sidecar.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		root.Close()
		return nil, err
	}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{}); err != nil {
		file.Close()
		root.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, ErrInUse
		}
		return nil, fmt.Errorf("acquire desktop instance lock: %w", err)
	}
	guard := &Guard{root: root, file: file}
	if err := guard.Verify(); err != nil {
		guard.Close()
		return nil, err
	}
	return guard, nil
}

// Verify 在使用实例所有权前核对原文件身份仍是固定目录里的锁文件。
func (g *Guard) Verify() error {
	if g == nil {
		return errors.New("desktop instance lock is unavailable")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.verifyLocked()
}

// WithOwnership 核验同一 app 根，并在整个恢复操作期间禁止本句柄关闭。
// operation 不得重入 Guard；返回的根只标识本次已核验的宿主目录。
func (g *Guard) WithOwnership(operation func(string) error) error {
	if g == nil || operation == nil {
		return errors.New("desktop recovery ownership is unavailable")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.verifyLocked(); err != nil {
		return err
	}
	return operation(g.root.Name())
}

func (g *Guard) verifyLocked() error {
	if g.file == nil {
		return errors.New("desktop instance lock is closed")
	}
	fresh, err := confinedfs.Open(g.root.Name())
	if err != nil {
		return err
	}
	defer fresh.Close()
	currentRoot, err := fresh.Stat(".")
	if err != nil {
		return err
	}
	originalRoot, err := g.root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(currentRoot, originalRoot) {
		return errors.New("desktop instance directory identity changed")
	}
	// 单独打开完成 confinedfs 的 reparse/hardlink 校验；字节锁属于原句柄，
	// 关闭核验句柄不会解除原实例锁。
	observed, err := g.root.OpenFileNoSymlink("sidecar.lock", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer observed.Close()
	current, err := observed.Stat()
	if err != nil {
		return err
	}
	original, err := g.file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(current, original) {
		return errors.New("desktop instance lock identity changed")
	}
	return nil
}

func (g *Guard) Close() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.file == nil {
		return g.closeErr
	}
	// 只关闭自己的描述符；不删除锁文件，也不按 PID 终止持有者。
	g.closeErr = errors.Join(g.file.Close(), g.root.Close())
	g.file = nil
	g.root = nil
	return g.closeErr
}
