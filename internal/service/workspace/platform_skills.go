// INPUT: 产品随附的 skills 目录与全局配置根。
// OUTPUT: nxs/Claude 共用的全局平台 Skill 兼容库。
// POS: workspace 与 runtime 装配之间的平台 Skill 同步边界。
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

type skillLibrarySyncState struct {
	sync.Mutex
	root        string
	fingerprint string
}

var platformSkillLibraryState skillLibrarySyncState

const skillLibraryManifestName = ".nexus-skill-library-manifest"

// EnsurePlatformSkillLibrary 将产品随附 Skill 同步到全局兼容根。
func EnsurePlatformSkillLibrary() error {
	return ensureCompatibleSkillLibrary(
		filepath.Join(appfs.Root(), "skills"),
		appfs.PlatformSkillRoot(),
		&platformSkillLibraryState,
	)
}

// ensureCompatibleSkillLibrary 把单一 Skill 源原子发布成 nxs/Claude 共用根。
func ensureCompatibleSkillLibrary(
	sourceRoot string,
	targetRoot string,
	state *skillLibrarySyncState,
) error {
	if _, err := os.Stat(sourceRoot); os.IsNotExist(err) {
		return clearCompatibleSkillLibrary(targetRoot, state)
	} else if err != nil {
		return err
	}
	fingerprint, err := skillLibraryFingerprint(sourceRoot)
	if err != nil {
		return err
	}

	state.Lock()
	defer state.Unlock()
	if state.root == targetRoot &&
		state.fingerprint == fingerprint &&
		skillLibraryReady(targetRoot, fingerprint) {
		return nil
	}
	if err = replaceCompatibleSkillLibrary(sourceRoot, targetRoot, fingerprint); err != nil {
		return err
	}
	state.root = targetRoot
	state.fingerprint = fingerprint
	return nil
}

func clearCompatibleSkillLibrary(
	targetRoot string,
	state *skillLibrarySyncState,
) error {
	state.Lock()
	defer state.Unlock()
	parentPath := filepath.Clean(filepath.Dir(targetRoot))
	parentRoot, err := confinedfs.Open(parentPath)
	if os.IsNotExist(err) {
		state.root = ""
		state.fingerprint = ""
		return nil
	}
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	if err = parentRoot.RemoveAll(filepath.Base(filepath.Clean(targetRoot))); err != nil &&
		!os.IsNotExist(err) {
		return err
	}
	state.root = ""
	state.fingerprint = ""
	return nil
}

func skillLibraryFingerprint(root string) (string, error) {
	source, err := confinedfs.Open(root)
	if err != nil {
		return "", err
	}
	defer source.Close()
	digest := sha256.New()
	if err := fingerprintSkillTree(source, "", digest); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func fingerprintSkillTree(root *confinedfs.Root, prefix string, digest hash.Hash) error {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})
	for _, entry := range entries {
		name := entry.Name()
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return confinedfs.ErrSymlink
		}
		relative := name
		if prefix != "" {
			relative = path.Join(prefix, name)
		}
		if info.IsDir() {
			child, err := root.OpenRootNoSymlink(name)
			if err != nil {
				return err
			}
			childErr := fingerprintSkillTree(child, relative, digest)
			closeErr := child.Close()
			if childErr != nil {
				return childErr
			}
			if closeErr != nil {
				return closeErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("platform skill source contains a special file: %s", relative)
		}
		file, err := root.OpenFileNoSymlink(name, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		openedInfo, err := file.Stat()
		if err != nil {
			file.Close()
			return err
		}
		_, _ = digest.Write([]byte(filepath.ToSlash(relative)))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(fmt.Sprintf("%o", openedInfo.Mode().Perm())))
		_, _ = digest.Write([]byte{0})
		_, err = io.Copy(digest, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func skillLibraryReady(root string, fingerprint string) bool {
	platformRoot, err := confinedfs.Open(root)
	if err != nil {
		return false
	}
	defer platformRoot.Close()
	if !skillDirectoryReadable(platformRoot) {
		return false
	}

	manifestFile, err := platformRoot.OpenFileNoSymlink(skillLibraryManifestName, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	manifestInfo, statErr := manifestFile.Stat()
	if statErr != nil ||
		!manifestInfo.Mode().IsRegular() ||
		manifestInfo.Mode().Perm()&0o004 == 0 {
		manifestFile.Close()
		return false
	}
	manifest, readErr := io.ReadAll(io.LimitReader(manifestFile, int64(len(fingerprint)+2)))
	closeErr := manifestFile.Close()
	if readErr != nil || closeErr != nil || string(manifest) != fingerprint+"\n" {
		return false
	}

	agentsRoot, err := platformRoot.OpenRootNoSymlink(".agents")
	if err != nil {
		return false
	}
	agentsReady := skillDirectoryReady(agentsRoot, "skills", "")
	agentsRoot.Close()
	if !agentsReady {
		return false
	}

	claudeRoot, err := platformRoot.OpenRootNoSymlink(".claude")
	if err != nil {
		return false
	}
	claudeReady := skillDirectoryReady(claudeRoot, "skills", "../.agents/skills")
	claudeRoot.Close()
	return claudeReady
}

func skillDirectoryReady(root *confinedfs.Root, name string, symlinkTarget string) bool {
	if !skillDirectoryReadable(root) {
		return false
	}
	info, err := root.Lstat(name)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if symlinkTarget == "" {
			return false
		}
		target, err := root.Readlink(name)
		if err != nil || filepath.ToSlash(filepath.Clean(target)) != symlinkTarget {
			return false
		}
		return true
	}
	if !info.IsDir() {
		return false
	}
	child, err := root.OpenRootNoSymlink(name)
	if err != nil {
		return false
	}
	ready := skillDirectoryReadable(child) && skillTreeReady(child)
	closeErr := child.Close()
	return ready && closeErr == nil
}

func skillDirectoryReadable(root *confinedfs.Root) bool {
	if root == nil {
		return false
	}
	info, err := root.Stat(".")
	if err != nil {
		return false
	}
	return info.IsDir() && info.Mode().Perm()&0o005 == 0o005
}

func skillTreeReady(root *confinedfs.Root) bool {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		info, err := root.Lstat(entry.Name())
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if info.IsDir() {
			if !skillDirectoryReadable(root) {
				return false
			}
			child, err := root.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return false
			}
			ready := skillDirectoryReadable(child) && skillTreeReady(child)
			closeErr := child.Close()
			if !ready || closeErr != nil {
				return false
			}
			continue
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o004 == 0 {
			return false
		}
	}
	return true
}

func replaceCompatibleSkillLibrary(sourceRoot string, targetRoot string, fingerprint string) error {
	parentPath := filepath.Clean(filepath.Dir(targetRoot))
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		return err
	}
	parentFS, err := confinedfs.Open(parentPath)
	if err != nil {
		return err
	}
	defer parentFS.Close()
	temporaryRelative, err := parentFS.MkdirTemp(".", ".skill-library-", 0o755)
	if err != nil {
		return err
	}
	temporaryRoot := filepath.Join(parentPath, filepath.FromSlash(temporaryRelative))
	cleanupTemporary := true
	defer func() {
		if cleanupTemporary {
			_ = parentFS.RemoveAll(temporaryRelative)
		}
	}()
	// 暂存目录与最终根处在同一父目录；先赋予只读穿越位，避免已有 runtime
	// 在原子替换窗口保留旧解析路径时因 MkdirTemp 默认 0700 被拒绝读取。
	temporaryFS, err := parentFS.OpenRootNoSymlink(temporaryRelative)
	if err != nil {
		return err
	}
	if err = temporaryFS.ChmodRoot(0o755); err != nil {
		temporaryFS.Close()
		return err
	}
	agentsFS, err := temporaryFS.OpenOrCreateRootNoSymlink(".agents/skills", 0o755)
	if err != nil {
		temporaryFS.Close()
		return err
	}
	sourceFS, err := confinedfs.Open(sourceRoot)
	if err != nil {
		agentsFS.Close()
		temporaryFS.Close()
		return err
	}
	err = copyRuntimeReadableSkillTree(agentsFS, sourceFS)
	sourceFS.Close()
	agentsFS.Close()
	if err != nil {
		temporaryFS.Close()
		return err
	}
	claudeSkillsRoot := filepath.Join(temporaryRoot, ".claude", "skills")
	if err := ensureRelativeSymlink(
		temporaryRoot,
		claudeSkillsRoot,
		filepath.Join("..", ".agents", "skills"),
	); err != nil {
		agentsFS, openErr := temporaryFS.OpenRootNoSymlink(".agents/skills")
		if openErr != nil {
			temporaryFS.Close()
			return openErr
		}
		claudeFS, createErr := temporaryFS.OpenOrCreateRootNoSymlink(".claude/skills", 0o755)
		if createErr != nil {
			agentsFS.Close()
			temporaryFS.Close()
			return createErr
		}
		copyErr := copyRuntimeReadableSkillTree(claudeFS, agentsFS)
		agentsFS.Close()
		claudeFS.Close()
		if copyErr != nil {
			temporaryFS.Close()
			return fmt.Errorf("创建 Claude Skill 入口失败: %w；镜像目录也失败: %v", err, copyErr)
		}
	}
	if err := temporaryFS.WriteFileAtomic(skillLibraryManifestName, []byte(fingerprint+"\n"), 0o644); err != nil {
		temporaryFS.Close()
		return err
	}
	// 平台包永远是 runtime 的只读资源；无条件归一化避免运行模式切换或
	// 存量文件 mode 导致平台 Skill 根在发布后不可读。
	if err := normalizeRuntimeReadableTree(temporaryFS); err != nil {
		temporaryFS.Close()
		return err
	}
	if err := temporaryFS.Close(); err != nil {
		return err
	}
	cleanupTemporary = false
	if err := replaceDirectory(temporaryRoot, targetRoot); err != nil {
		_ = parentFS.RemoveAll(temporaryRelative)
		return err
	}
	return nil
}

// copyRuntimeReadableSkillTree 在复制时直接投影 runtime 需要的权限。
// Windows 会把只读 mode 映射成文件属性，不能先照搬再通过目录句柄恢复。
func copyRuntimeReadableSkillTree(target *confinedfs.Root, source *confinedfs.Root) error {
	entries, err := fs.ReadDir(source.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = copyRuntimeReadableSkillEntry(target, source, entry.Name()); err != nil {
			return err
		}
	}
	return target.ChmodRoot(0o755)
}

func copyRuntimeReadableSkillEntry(
	target *confinedfs.Root,
	source *confinedfs.Root,
	name string,
) error {
	info, err := source.Lstat(name)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return confinedfs.ErrSymlink
	}
	if info.IsDir() {
		sourceChild, err := source.OpenRootNoSymlink(name)
		if err != nil {
			return err
		}
		targetChild, createErr := target.OpenOrCreateRootNoSymlink(name, 0o755)
		if createErr != nil {
			sourceChild.Close()
			return createErr
		}
		copyErr := copyRuntimeReadableSkillTree(targetChild, sourceChild)
		sourceCloseErr := sourceChild.Close()
		targetCloseErr := targetChild.Close()
		if copyErr != nil {
			return copyErr
		}
		if sourceCloseErr != nil {
			return sourceCloseErr
		}
		return targetCloseErr
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("skill source contains a special file: %s", name)
	}
	sourceFile, err := source.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	openedInfo, err := sourceFile.Stat()
	if err != nil {
		sourceFile.Close()
		return err
	}
	mode := os.FileMode(0o644)
	if openedInfo.Mode().Perm()&0o111 != 0 {
		mode = 0o755
	}
	copyErr := target.WriteFileAtomicFrom(name, sourceFile, mode)
	closeErr := sourceFile.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func normalizeRuntimeReadableTree(root *confinedfs.Root) error {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, err := root.Lstat(entry.Name())
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			child, err := root.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return err
			}
			normalizeErr := normalizeRuntimeReadableTree(child)
			child.Close()
			if normalizeErr != nil {
				return normalizeErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		file, err := root.OpenFileNoSymlink(entry.Name(), os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		err = file.Chmod(mode)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return root.ChmodRoot(0o755)
}

func replaceDirectory(sourceRoot string, targetRoot string) error {
	sourceParent := filepath.Clean(filepath.Dir(sourceRoot))
	targetParent := filepath.Clean(filepath.Dir(targetRoot))
	if sourceParent != targetParent {
		return fmt.Errorf("atomic directory replacement requires a shared parent")
	}
	return replaceDirectoryWithin(targetParent, sourceRoot, targetRoot)
}

func replaceDirectoryWithin(boundaryRoot string, sourceRoot string, targetRoot string) error {
	if err := os.MkdirAll(boundaryRoot, 0o755); err != nil {
		return err
	}
	root, err := confinedfs.Open(boundaryRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	sourceRelative, err := relativePathWithin(boundaryRoot, sourceRoot)
	if err != nil {
		return err
	}
	targetRelative, err := relativePathWithin(boundaryRoot, targetRoot)
	if err != nil {
		return err
	}
	return replaceDirectoryWithinRoot(root, sourceRelative, targetRelative)
}

func replaceDirectoryWithinRoot(
	root *confinedfs.Root,
	sourceRelative string,
	targetRelative string,
) error {
	var err error
	if _, err = root.Lstat(targetRelative); os.IsNotExist(err) {
		return root.Rename(sourceRelative, targetRelative)
	} else if err != nil {
		return err
	}
	backupRelative := targetRelative + ".old"
	if err = root.RemoveAll(backupRelative); err != nil {
		return err
	}
	if err = root.Rename(targetRelative, backupRelative); err != nil {
		return err
	}
	if err = root.Rename(sourceRelative, targetRelative); err != nil {
		_ = root.Rename(backupRelative, targetRelative)
		return err
	}
	return root.RemoveAll(backupRelative)
}
