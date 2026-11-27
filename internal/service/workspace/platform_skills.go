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
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// 平台 Skill 只在启动时发布一次；互斥只防止测试等并发调用交错替换同一根。
var platformSkillLibraryMu sync.Mutex

const skillLibraryManifestName = ".nexus-skill-library-manifest"

const skillLibraryStagingPrefix = ".skill-library-"

// 发布进程中途被杀时 defer 来不及清理暂存目录；只回收足够旧的，避免删掉另一个进程正在写的暂存。
const staleSkillLibraryStagingAge = time.Hour

// EnsurePlatformSkillLibrary 将产品随附 Skill 同步到全局兼容根。
func EnsurePlatformSkillLibrary() error {
	return ensureCompatibleSkillLibrary(
		filepath.Join(appfs.Root(), "skills"),
		appfs.PlatformSkillRoot(),
	)
}

// ensureCompatibleSkillLibrary 把单一 Skill 源原子发布成 nxs/Claude 共用根。
// 已发布内容与源指纹一致时跳过重发：投影仅宿主可写，内容校验足以替代每次启动整库复制。
func ensureCompatibleSkillLibrary(sourceRoot string, targetRoot string) error {
	platformSkillLibraryMu.Lock()
	defer platformSkillLibraryMu.Unlock()
	removeStaleSkillLibraryStaging(filepath.Clean(filepath.Dir(targetRoot)), time.Now())
	if _, err := os.Stat(sourceRoot); os.IsNotExist(err) {
		return clearCompatibleSkillLibrary(targetRoot)
	} else if err != nil {
		return err
	}
	fingerprint, err := skillLibraryFingerprint(sourceRoot)
	if err != nil {
		return err
	}
	if skillLibraryReady(targetRoot, fingerprint) && publishedSkillTreesMatch(targetRoot, fingerprint) {
		return nil
	}
	return replaceCompatibleSkillLibrary(sourceRoot, targetRoot, fingerprint)
}

// removeStaleSkillLibraryStaging 尽力回收遗留暂存目录；失败只留下垃圾，不影响发布。
func removeStaleSkillLibraryStaging(parentPath string, now time.Time) {
	parentRoot, err := confinedfs.Open(parentPath)
	if err != nil {
		return
	}
	defer parentRoot.Close()
	entries, err := fs.ReadDir(parentRoot.FS(), ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), skillLibraryStagingPrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) < staleSkillLibraryStagingAge {
			continue
		}
		_ = parentRoot.RemoveAll(entry.Name())
	}
}

// publishedSkillTreesMatch 校验已发布的真实目录内容；Claude 入口为符号链接时由 skillLibraryReady 校验指向。
func publishedSkillTreesMatch(targetRoot string, fingerprint string) bool {
	for _, relative := range []string{".agents/skills", ".claude/skills"} {
		published := filepath.Join(targetRoot, filepath.FromSlash(relative))
		info, err := os.Lstat(published)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		digest, err := skillLibraryFingerprint(published)
		if err != nil || digest != fingerprint {
			return false
		}
	}
	return true
}

func clearCompatibleSkillLibrary(targetRoot string) error {
	parentPath := filepath.Clean(filepath.Dir(targetRoot))
	parentRoot, err := confinedfs.Open(parentPath)
	if os.IsNotExist(err) {
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
		_, _ = fmt.Fprintf(digest, "%o", runtimeReadableFileMode(openedInfo.Mode()))
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
	temporaryRelative, err := parentFS.MkdirTemp(".", skillLibraryStagingPrefix, 0o755)
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
	copyErr := target.WriteFileAtomicFrom(name, sourceFile, runtimeReadableFileMode(openedInfo.Mode()))
	closeErr := sourceFile.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// runtimeReadableFileMode 是发布投影的唯一权限规则：只保留可执行位，其余统一只读可读。
// 指纹也按投影权限计算，使源目录与已发布目录可以直接比较。
func runtimeReadableFileMode(mode os.FileMode) os.FileMode {
	if mode.Perm()&0o111 != 0 {
		return 0o755
	}
	return 0o644
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
		err = file.Chmod(runtimeReadableFileMode(info.Mode()))
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
