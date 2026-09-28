//go:build darwin

// INPUT: 两个已固定的目录根、单段条目名与原目录身份。
// OUTPUT: 不覆盖目标的原子移动及移动后身份复核；失败不删除任何条目。
// POS: macOS 宿主回收区原语；调用方持久化意图并保护目标根。
package confinedfs

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// QuarantineDirectory moves a single directory between pinned roots without
// replacing an existing destination. A post-move error may leave the entry at
// destination: callers must reconcile there, never repeat using a new name.
// The destination must be inaccessible to tasks throughout verification/removal.
func (r *Root) QuarantineDirectory(name string, destination *Root, target string, expected os.FileInfo) error {
	if r == nil || destination == nil || r.root == nil || destination.root == nil {
		return errors.New("quarantine requires fixed directory roots")
	}
	for _, n := range []string{name, target} {
		if n == "" || n == "." || n == ".." || strings.ContainsAny(n, "/\\\x00") {
			return errors.New("quarantine requires single path components")
		}
	}
	if expected == nil || !expected.IsDir() || expected.Mode()&os.ModeSymlink != 0 {
		return errors.New("quarantine requires original directory identity")
	}
	observed, err := r.Lstat(name)
	if err != nil {
		return err
	}
	if !os.SameFile(expected, observed) || !observed.IsDir() || observed.Mode()&os.ModeSymlink != 0 {
		return ErrChanged
	}
	sourceFD, err := r.root.Open(".")
	if err != nil {
		return err
	}
	defer sourceFD.Close()
	targetFD, err := destination.root.Open(".")
	if err != nil {
		return err
	}
	defer targetFD.Close()
	if err := unix.RenameatxNp(int(sourceFD.Fd()), name, int(targetFD.Fd()), target, unix.RENAME_EXCL); err != nil {
		return err
	}
	observed, err = destination.Lstat(target)
	if err != nil {
		return err
	}
	if !os.SameFile(expected, observed) || !observed.IsDir() || observed.Mode()&os.ModeSymlink != 0 {
		return ErrChanged
	}
	return errors.Join(sourceFD.Sync(), targetFD.Sync())
}

// SyncDirectory makes directory-entry mutations durable before a host receipt
// advances. Sync failure leaves the receipt pending even if the mutation ran.
func (r *Root) SyncDirectory() error {
	if r == nil || r.root == nil {
		return errors.New("directory root unavailable")
	}
	file, err := r.root.Open(".")
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
