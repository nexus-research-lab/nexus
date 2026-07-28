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

var platformSkillLibraryState struct {
	sync.Mutex
	root        string
	fingerprint string
}

const platformSkillManifestName = ".platform-skill-manifest"

// EnsurePlatformSkillLibrary 将产品随附 Skill 同步到全局兼容根。
//
// 同步只发生一次内容变化时，Agent workspace 不再持有 Skill 副本；nxs 和
// Claude 通过两个兼容入口读取同一份平台文件，更新平台文件后下一次同步即可生效。
func EnsurePlatformSkillLibrary() error {
	sourceRoot := filepath.Join(appfs.Root(), "skills")
	if _, err := os.Stat(sourceRoot); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	fingerprint, err := platformSkillFingerprint(sourceRoot)
	if err != nil {
		return err
	}
	targetRoot := appfs.PlatformSkillRoot()

	platformSkillLibraryState.Lock()
	defer platformSkillLibraryState.Unlock()
	if platformSkillLibraryState.root == targetRoot && platformSkillLibraryState.fingerprint == fingerprint && platformSkillLibraryReady(targetRoot, fingerprint) {
		return nil
	}
	if err = replacePlatformSkillLibrary(sourceRoot, targetRoot, fingerprint); err != nil {
		return err
	}
	platformSkillLibraryState.root = targetRoot
	platformSkillLibraryState.fingerprint = fingerprint
	return nil
}

func platformSkillFingerprint(root string) (string, error) {
	source, err := confinedfs.Open(root)
	if err != nil {
		return "", err
	}
	defer source.Close()

	hash := sha256.New()
	if err := fingerprintPlatformSkillTree(source, "", hash); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func fingerprintPlatformSkillTree(root *confinedfs.Root, prefix string, digest hash.Hash) error {
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
			childErr := fingerprintPlatformSkillTree(child, relative, digest)
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

func platformSkillLibraryReady(root string, fingerprint string) bool {
	platformRoot, err := confinedfs.Open(root)
	if err != nil {
		return false
	}
	defer platformRoot.Close()
	if !platformSkillDirectoryReadable(platformRoot) {
		return false
	}

	manifestFile, err := platformRoot.OpenFileNoSymlink(platformSkillManifestName, os.O_RDONLY, 0)
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
	agentsReady := platformSkillDirectoryReady(agentsRoot, "skills", "")
	agentsRoot.Close()
	if !agentsReady {
		return false
	}

	claudeRoot, err := platformRoot.OpenRootNoSymlink(".claude")
	if err != nil {
		return false
	}
	claudeReady := platformSkillDirectoryReady(claudeRoot, "skills", "../.agents/skills")
	claudeRoot.Close()
	return claudeReady
}

func platformSkillDirectoryReady(root *confinedfs.Root, name string, symlinkTarget string) bool {
	if !platformSkillDirectoryReadable(root) {
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
	ready := platformSkillDirectoryReadable(child) && platformSkillTreeReady(child)
	closeErr := child.Close()
	return ready && closeErr == nil
}

func platformSkillDirectoryReadable(root *confinedfs.Root) bool {
	if root == nil {
		return false
	}
	info, err := root.Stat(".")
	if err != nil {
		return false
	}
	return info.IsDir() && info.Mode().Perm()&0o005 == 0o005
}

func platformSkillTreeReady(root *confinedfs.Root) bool {
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
			if !platformSkillDirectoryReadable(root) {
				return false
			}
			child, err := root.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return false
			}
			ready := platformSkillDirectoryReadable(child) && platformSkillTreeReady(child)
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

func replacePlatformSkillLibrary(sourceRoot string, targetRoot string, fingerprint string) error {
	parentPath := filepath.Clean(filepath.Dir(targetRoot))
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		return err
	}
	parentFS, err := confinedfs.Open(parentPath)
	if err != nil {
		return err
	}
	defer parentFS.Close()
	temporaryRelative, err := parentFS.MkdirTemp(".", ".platform-skills-", 0o755)
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
	err = agentsFS.CopyTreeFrom(sourceFS)
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
		copyErr := claudeFS.CopyTreeFrom(agentsFS)
		agentsFS.Close()
		claudeFS.Close()
		if copyErr != nil {
			temporaryFS.Close()
			return fmt.Errorf("创建 Claude Skill 入口失败: %w；镜像目录也失败: %v", err, copyErr)
		}
	}
	if err := temporaryFS.WriteFileAtomic(platformSkillManifestName, []byte(fingerprint+"\n"), 0o644); err != nil {
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
