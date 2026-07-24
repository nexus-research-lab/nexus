// 该文件实现以目录文件描述符为边界的文件访问门面。
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

// ReadFile 在根目录内读取文件。
func (r *Root) ReadFile(name string) ([]byte, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	return r.root.ReadFile(name)
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

// OpenFile 在根目录内打开文件。
func (r *Root) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	return r.root.OpenFile(name, flag, perm)
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
	return r.root.MkdirAll(name, perm)
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
	if err = r.MkdirAll(parent, perm); err != nil {
		return "", err
	}
	for attempt := 0; attempt < 16; attempt++ {
		suffix, randomErr := randomSuffix()
		if randomErr != nil {
			return "", randomErr
		}
		name := path.Join(parent, prefix+suffix)
		if err = r.Mkdir(name, perm); errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return name, nil
	}
	return "", errors.New("unable to allocate confined temporary directory")
}

// WriteFileAtomic 通过同一根目录内的临时文件和 rename 原子替换文件。
func (r *Root) WriteFileAtomic(name string, data []byte, perm os.FileMode) error {
	name, err := normalize(name)
	if err != nil {
		return err
	}
	if name == "." {
		return errors.New("cannot write confined root")
	}
	parent := path.Dir(name)
	if err = r.MkdirAll(parent, 0o770); err != nil {
		return err
	}

	var temporaryName string
	var file *os.File
	for attempt := 0; attempt < 16; attempt++ {
		suffix, randomErr := randomSuffix()
		if randomErr != nil {
			return randomErr
		}
		temporaryName = path.Join(parent, ".nexus-confined-"+suffix+".tmp")
		file, err = r.OpenFile(
			temporaryName,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			perm,
		)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	if file == nil {
		return errors.New("unable to allocate confined temporary file")
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = r.root.Remove(temporaryName)
		}
	}()

	if _, err = io.Copy(file, bytes.NewReader(data)); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = r.root.Rename(temporaryName, name); err != nil {
		return err
	}
	committed = true
	return nil
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
