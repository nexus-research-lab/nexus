// INPUT: 经 owner/workspace 校验的相对路径、文件内容与可选旧正文。
// OUTPUT: 以目录 fd 为边界的读写、原子替换和替换前内容复核。
// POS: 宿主 confined filesystem 门面；不解释业务 revision 或身份。
package confinedfs

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrAbsolutePath 表示调用方传入了不允许的绝对路径。
	ErrAbsolutePath = errors.New("confined path must be relative")
	// ErrParentTraversal 表示调用方传入了显式父目录遍历。
	ErrParentTraversal = errors.New("confined path contains parent traversal")
	// ErrNUL 表示路径包含 NUL 字节。
	ErrNUL = errors.New("confined path contains NUL")
	// ErrSymlink 表示 no-symlink API 遇到了符号链接。
	ErrSymlink = errors.New("confined path contains symlink")
	// ErrChanged 表示校验后的路径条目在打开期间被替换。
	ErrChanged = errors.New("confined path changed while opening")
	// ErrHardlink 表示受保护文件还有其他目录项指向同一 inode。
	ErrHardlink = errors.New("confined file has multiple hard links")
)

// Root 将一棵已打开的目录树封装为受限文件系统。
type Root struct {
	root *os.Root
	name string
}

// Open 打开并固定目录根。最终路径段必须是稳定的真实目录；符号链接或
// Lstat/OpenRoot 之间被替换的 inode 会被拒绝，避免宿主把攻击者准备的链接
// 目标误当成已经授权的根。
func Open(name string) (*Root, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("confined root is empty")
	}
	name = filepath.Clean(name)
	expected, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if expected.Mode()&os.ModeSymlink != 0 || !expected.IsDir() {
		return nil, errors.New("confined root must be a real directory")
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if !opened.IsDir() || !os.SameFile(expected, opened) {
		_ = root.Close()
		return nil, errors.New("confined root changed while opening")
	}
	return &Root{root: root, name: name}, nil
}

// RemoveTree 删除指定目录树的最后一个路径段。
//
// 调用方应只把宿主已授权的顶层目录传入；父目录以目录 fd 固定后，
// 最后一个段的替换不会跟随到父树之外。
func RemoveTree(name string) error {
	name = filepath.Clean(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return errors.New("cannot remove broad root")
	}
	parent := filepath.Dir(name)
	base := filepath.Base(name)
	root, err := Open(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(base)
}

// Name 返回打开根目录时使用的宿主路径，仅用于桌面端展示或日志。
func (r *Root) Name() string {
	if r == nil {
		return ""
	}
	return r.name
}

// Close 关闭底层目录文件描述符。
func (r *Root) Close() error {
	if r == nil || r.root == nil {
		return nil
	}
	return r.root.Close()
}

// ReadFile 在根目录内读取真实普通文件，拒绝路径中的符号链接与硬链接。
func (r *Root) ReadFile(name string) ([]byte, error) {
	file, err := r.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// Stat 在根目录内获取文件信息。
func (r *Root) Stat(name string) (os.FileInfo, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	return r.root.Stat(name)
}

// Lstat 在根目录内获取目录项本身的信息。
func (r *Root) Lstat(name string) (os.FileInfo, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	return r.root.Lstat(name)
}

// FS 返回受限根对应的 fs.FS 视图。
func (r *Root) FS() fs.FS {
	if r == nil || r.root == nil {
		return nil
	}
	return r.root.FS()
}

// Open 以只读方式打开根目录内的文件。
func (r *Root) Open(name string) (*os.File, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	return r.root.Open(name)
}

// OpenRoot 打开根目录内的子目录，并继续保留目录 fd 边界。
func (r *Root) OpenRoot(name string) (*Root, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	child, err := r.root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	return &Root{
		root: child,
		name: filepath.Join(r.name, filepath.FromSlash(name)),
	}, nil
}

// OpenRootNoSymlink 逐级打开真实目录，拒绝任意路径段中的符号链接。
//
// 每一级都比较 Lstat 与打开后的 inode；即使条目在两次系统调用之间被替换，
// 返回的目录 fd 也只能指向已校验的同一目录。
func (r *Root) OpenRootNoSymlink(name string) (*Root, error) {
	return r.openRootNoSymlink(name, false, 0)
}

// OpenOrCreateRootNoSymlink 逐级创建并打开真实目录，拒绝符号链接。
func (r *Root) OpenOrCreateRootNoSymlink(name string, perm os.FileMode) (*Root, error) {
	if perm&0o777 != perm {
		return nil, errors.New("unsupported directory mode")
	}
	return r.openRootNoSymlink(name, true, perm)
}

func (r *Root) openRootNoSymlink(
	name string,
	create bool,
	perm os.FileMode,
) (*Root, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	current, err := r.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	if name == "." {
		return current, nil
	}
	for _, component := range strings.Split(name, "/") {
		expected, statErr := current.Lstat(component)
		if errors.Is(statErr, os.ErrNotExist) && create {
			if mkdirErr := current.Mkdir(component, perm); mkdirErr != nil &&
				!errors.Is(mkdirErr, fs.ErrExist) {
				current.Close()
				return nil, mkdirErr
			}
			expected, statErr = current.Lstat(component)
		}
		if statErr != nil {
			current.Close()
			return nil, statErr
		}
		if expected.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, ErrSymlink
		}
		if !expected.IsDir() {
			current.Close()
			return nil, errors.New("confined path component is not a directory")
		}

		next, openErr := current.OpenRoot(component)
		if openErr != nil {
			current.Close()
			return nil, openErr
		}
		opened, openErr := next.Stat(".")
		observed, observeErr := current.Lstat(component)
		if openErr != nil ||
			observeErr != nil ||
			observed.Mode()&os.ModeSymlink != 0 ||
			!os.SameFile(expected, opened) ||
			!os.SameFile(expected, observed) {
			next.Close()
			current.Close()
			if openErr != nil {
				return nil, openErr
			}
			if observeErr != nil {
				return nil, observeErr
			}
			return nil, ErrChanged
		}
		current.Close()
		current = next
	}
	return current, nil
}

// OpenFile 在根目录内打开文件。
func (r *Root) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	return r.root.OpenFile(name, flag, perm)
}

// OpenFileNoSymlink 打开真实普通文件，拒绝任意路径段的 symlink、最终文件
// 的硬链接与替换竞态。
//
// 该入口不接受 O_TRUNC；调用方应使用原子临时文件 + rename 完成替换。
func (r *Root) OpenFileNoSymlink(name string, flag int, perm os.FileMode) (*os.File, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	if name == "." {
		return nil, errors.New("confined file path is root")
	}
	if flag&os.O_TRUNC != 0 {
		return nil, errors.New("OpenFileNoSymlink does not allow O_TRUNC")
	}
	parent, err := r.OpenRootNoSymlink(path.Dir(name))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return parent.openFileNoSymlink(path.Base(name), flag, perm)
}

func (r *Root) openFileNoSymlink(name string, flag int, perm os.FileMode) (*os.File, error) {
	for attempt := 0; attempt < 3; attempt++ {
		expected, statErr := r.Lstat(name)
		if errors.Is(statErr, os.ErrNotExist) && flag&os.O_CREATE != 0 {
			file, createErr := r.OpenFile(name, flag|os.O_EXCL, perm)
			if errors.Is(createErr, fs.ErrExist) {
				continue
			}
			return file, createErr
		}
		if statErr != nil {
			return nil, statErr
		}
		if expected.Mode()&os.ModeSymlink != 0 {
			return nil, ErrSymlink
		}
		if !expected.Mode().IsRegular() {
			return nil, errors.New("confined file path is not a regular file")
		}
		if hasMultipleHardLinks(expected) {
			return nil, ErrHardlink
		}

		file, openErr := r.OpenFile(name, flag&^os.O_CREATE, perm)
		if errors.Is(openErr, os.ErrNotExist) && flag&os.O_CREATE != 0 {
			continue
		}
		if openErr != nil {
			return nil, openErr
		}
		opened, openErr := file.Stat()
		observed, observeErr := r.Lstat(name)
		if openErr != nil ||
			observeErr != nil ||
			observed.Mode()&os.ModeSymlink != 0 ||
			hasMultipleHardLinks(opened) ||
			hasMultipleHardLinks(observed) ||
			!os.SameFile(expected, opened) ||
			!os.SameFile(expected, observed) {
			file.Close()
			if openErr != nil {
				return nil, openErr
			}
			if observeErr != nil {
				return nil, observeErr
			}
			if observed.Mode()&os.ModeSymlink != 0 {
				return nil, ErrSymlink
			}
			if hasMultipleHardLinks(opened) || hasMultipleHardLinks(observed) {
				return nil, ErrHardlink
			}
			// 合法的原子替换也会短暂改变 inode；下一轮仍会完整执行安全校验。
			continue
		}
		return file, nil
	}
	return nil, ErrChanged
}

// Readlink 读取根目录内的符号链接目标。
func (r *Root) Readlink(name string) (string, error) {
	name, err := normalize(name)
	if err != nil {
		return "", err
	}
	return r.root.Readlink(name)
}

// Symlink 在根目录内创建符号链接。
func (r *Root) Symlink(oldName string, newName string) error {
	newName, err := normalize(newName)
	if err != nil {
		return err
	}
	if newName == "." {
		return errors.New("cannot replace confined root")
	}
	return r.root.Symlink(oldName, newName)
}

// Chmod 修改根目录内条目的权限。
func (r *Root) Chmod(name string, mode os.FileMode) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	return r.root.Chmod(name, mode)
}

// MkdirAll 在根目录内创建目录树。
func (r *Root) MkdirAll(name string, perm os.FileMode) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	if name == "." {
		return nil
	}
	child, err := r.OpenOrCreateRootNoSymlink(name, perm)
	if err != nil {
		return err
	}
	return child.Close()
}

// Mkdir 在根目录内创建单个目录。
func (r *Root) Mkdir(name string, perm os.FileMode) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	if name == "." {
		return fs.ErrExist
	}
	return r.root.Mkdir(name, perm)
}

// MkdirTemp 在根目录内创建随机目录并返回相对路径。
func (r *Root) MkdirTemp(parent string, prefix string, perm os.FileMode) (string, error) {
	parent, err := normalize(parent)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(prefix, `/\`+"\x00") {
		return "", errors.New("temporary directory prefix contains a path separator")
	}
	parentRoot, err := r.OpenOrCreateRootNoSymlink(parent, perm)
	if err != nil {
		return "", err
	}
	defer parentRoot.Close()
	for attempt := 0; attempt < 16; attempt++ {
		suffix, randomErr := randomSuffix()
		if randomErr != nil {
			return "", randomErr
		}
		baseName := prefix + suffix
		if err = parentRoot.Mkdir(baseName, perm); errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return path.Join(parent, baseName), nil
	}
	return "", errors.New("unable to allocate confined temporary directory")
}

// WriteFileAtomic 通过同一根目录内的临时文件和 rename 原子替换文件。
func (r *Root) WriteFileAtomic(name string, data []byte, perm os.FileMode) error {
	return r.WriteFileAtomicFrom(name, bytes.NewReader(data), perm)
}

// WriteFileAtomicFrom 从 reader 流式写入临时文件，再原子替换目标文件。
func (r *Root) WriteFileAtomicFrom(name string, source io.Reader, perm os.FileMode) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	if name == "." {
		return errors.New("cannot write confined root")
	}
	parent := path.Dir(name)
	parentRoot, err := r.OpenOrCreateRootNoSymlink(parent, 0o770)
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	_, err = parentRoot.writeFileAtomic(path.Base(name), source, perm, nil, false)
	return err
}

// WriteFileAtomicIfContent 先完整写入并同步临时文件，再在最终 rename 前
// 复核目标正文。返回 false, nil 表示目标已变更且本次没有替换。
func (r *Root) WriteFileAtomicIfContent(
	name string,
	data []byte,
	expectedContent []byte,
	perm os.FileMode,
) (bool, error) {
	name, err := normalize(name)
	if err != nil {
		return false, err
	}
	if name == "." {
		return false, errors.New("cannot write confined root")
	}
	parent := path.Dir(name)
	parentRoot, err := r.OpenOrCreateRootNoSymlink(parent, 0o770)
	if err != nil {
		return false, err
	}
	defer parentRoot.Close()
	return parentRoot.writeFileAtomic(
		path.Base(name),
		bytes.NewReader(data),
		perm,
		expectedContent,
		true,
	)
}

func (r *Root) writeFileAtomic(
	name string,
	source io.Reader,
	perm os.FileMode,
	expectedContent []byte,
	conditional bool,
) (bool, error) {
	var temporaryName string
	var file *os.File
	var err error
	for attempt := 0; attempt < 16; attempt++ {
		suffix, randomErr := randomSuffix()
		if randomErr != nil {
			return false, randomErr
		}
		temporaryName = ".nexus-confined-" + suffix + ".tmp"
		file, err = r.OpenFile(
			temporaryName,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			perm,
		)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		break
	}
	if file == nil {
		return false, errors.New("unable to allocate confined temporary file")
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = r.root.Remove(temporaryName)
		}
	}()

	if _, err = io.Copy(file, source); err != nil {
		return false, err
	}
	if err = file.Chmod(perm.Perm()); err != nil {
		return false, err
	}
	if err = file.Sync(); err != nil {
		return false, err
	}
	if err = file.Close(); err != nil {
		return false, err
	}
	if conditional {
		currentContent, readErr := r.ReadFile(name)
		if errors.Is(readErr, fs.ErrNotExist) {
			return false, nil
		}
		if readErr != nil {
			return false, readErr
		}
		if !bytes.Equal(currentContent, expectedContent) {
			return false, nil
		}
	}
	if err = r.root.Rename(temporaryName, name); err != nil {
		return false, err
	}
	committed = true
	return true, nil
}

// ChmodRoot 通过已固定的目录句柄修改根目录自身权限。
func (r *Root) ChmodRoot(mode os.FileMode) error {
	if r == nil || r.root == nil {
		return errors.New("confined root is closed")
	}
	directory, err := r.root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Chmod(mode)
}

// CopyTreeFrom 把已固定源目录中的真实目录与普通文件复制到当前根。
//
// 源树中的符号链接、硬链接和特殊文件会被拒绝，目录与文件权限保持不变。
func (r *Root) CopyTreeFrom(source *Root) error {
	if r == nil || r.root == nil || source == nil || source.root == nil {
		return errors.New("confined copy root is closed")
	}
	sourceInfo, err := source.Stat(".")
	if err != nil {
		return err
	}
	entries, err := fs.ReadDir(source.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, err := source.Lstat(entry.Name())
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrSymlink
		}
		if info.IsDir() {
			sourceChild, err := source.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return err
			}
			targetChild, targetErr := r.OpenOrCreateRootNoSymlink(entry.Name(), 0o755)
			if targetErr != nil {
				sourceChild.Close()
				return targetErr
			}
			copyErr := targetChild.CopyTreeFrom(sourceChild)
			sourceChild.Close()
			targetChild.Close()
			if copyErr != nil {
				return copyErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return errors.New("confined source contains a special file")
		}
		sourceFile, err := source.OpenFileNoSymlink(entry.Name(), os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		openedInfo, err := sourceFile.Stat()
		if err != nil {
			sourceFile.Close()
			return err
		}
		copyErr := r.WriteFileAtomicFrom(entry.Name(), sourceFile, openedInfo.Mode().Perm())
		sourceFile.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return r.ChmodRoot(sourceInfo.Mode().Perm())
}

// Remove 删除根目录内的单个文件或空目录。
func (r *Root) Remove(name string) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	return r.root.Remove(name)
}

// RemoveAll 删除根目录内的文件或目录树。
func (r *Root) RemoveAll(name string) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	if name == "." {
		return errors.New("cannot remove confined root")
	}
	return r.root.RemoveAll(name)
}

// Rename 在同一根目录内原子移动条目。
func (r *Root) Rename(oldName string, newName string) error {
	oldName, err := normalize(oldName)
	if err != nil {
		return err
	}
	newName, err = normalize(newName)
	if err != nil {
		return err
	}
	if oldName == "." || newName == "." {
		return errors.New("cannot rename confined root")
	}
	return r.root.Rename(oldName, newName)
}

// Walk 在根目录内遍历。回调收到的路径均为 slash 分隔的相对路径。
func (r *Root) Walk(name string, callback fs.WalkDirFunc) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	return fs.WalkDir(r.root.FS(), name, func(relative string, entry fs.DirEntry, walkErr error) error {
		return callback(relative, entry, walkErr)
	})
}

func normalize(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, `\`, "/"))
	if name == "" {
		return "", errors.New("confined path is empty")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", ErrNUL
	}
	hostPath := filepath.FromSlash(name)
	if strings.HasPrefix(name, "/") ||
		filepath.IsAbs(hostPath) ||
		filepath.VolumeName(hostPath) != "" ||
		isWindowsDrivePath(name) {
		return "", ErrAbsolutePath
	}
	// os.Root 接受 "." 作为根本身；其余路径拒绝显式 ..，避免把
	// 业务层的路径归一化误判为授权。
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return "", ErrParentTraversal
		}
	}
	name = path.Clean(name)
	if name == ".." || strings.HasPrefix(name, "../") {
		return "", ErrParentTraversal
	}
	return name, nil
}

func isWindowsDrivePath(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	value := name[0]
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
}

func randomSuffix() (string, error) {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate confined temporary name: %w", err)
	}
	return hex.EncodeToString(bytes[:]) + "-" + fmt.Sprintf("%x", time.Now().UnixNano()), nil
}
