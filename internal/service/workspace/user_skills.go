// INPUT: 用户身份与 Nexus workspace 配置。
// OUTPUT: owner 共享 Skill 源、Claude 兼容入口与原子目录替换能力。
// POS: 外部 Skill 导入与 runtime 目录装配的文件边界。
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var userSkillLibraryLocks sync.Map

// UserSkillLibraryRoot 返回指定 owner 的共享 Skill 根。
//
// 直接复用 Agent workspace 的 owner 根，确保自定义 workspace 配置、owner
// 目录命名和系统 owner 的扁平布局保持一致。
func UserSkillLibraryRoot(cfg config.Config, ownerUserID string) string {
	return agentsvc.UserWorkspaceBasePath(cfg, ownerUserID)
}

// UserSkillDiscoveryRoot 返回 nxs/Claude 共同发现的用户 Skill 目录。
func UserSkillDiscoveryRoot(cfg config.Config, ownerUserID string) string {
	return filepath.Join(UserSkillLibraryRoot(cfg, ownerUserID), ".agents", "skills")
}

// SkillLibraryRoots 返回 runtime 需要读取的平台与用户全局 Skill 根。
func SkillLibraryRoots(cfg config.Config, ownerUserID string) []string {
	roots := []string{appfs.PlatformSkillRoot()}
	if strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		if info, err := os.Stat(appfs.HostSkillRoot()); err == nil && info.IsDir() {
			roots = append(roots, appfs.HostSkillRoot())
		}
	}
	return append(roots, UserSkillLibraryRoot(cfg, ownerUserID))
}

// EnsureUserSkillLibrary 确保用户外部 Skill 同时具备 nxs 与 Claude 发现入口。
func EnsureUserSkillLibrary(cfg config.Config, ownerUserID string) error {
	if err := syncUserSkillLibrary(cfg, ownerUserID, false); err != nil {
		return err
	}
	return EnsureHostSkillLibrary(cfg)
}

// RefreshUserSkillLibrary 在 owner 源变化后刷新 Claude fallback 镜像。
func RefreshUserSkillLibrary(cfg config.Config, ownerUserID string) error {
	if err := syncUserSkillLibrary(cfg, ownerUserID, true); err != nil {
		return err
	}
	return EnsureHostSkillLibrary(cfg)
}

func syncUserSkillLibrary(cfg config.Config, ownerUserID string, refreshMirror bool) error {
	root := UserSkillLibraryRoot(cfg, ownerUserID)
	lockValue, _ := userSkillLibraryLocks.LoadOrStore(root, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	confinedRoot, err := workspacestore.New(cfg.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		root,
		true,
	)
	if err != nil {
		return err
	}
	defer confinedRoot.Close()
	agentsDirectory, err := confinedRoot.OpenOrCreateRootNoSymlink(
		".agents/skills",
		workspaceDirectoryMode(),
	)
	if err != nil {
		return err
	}
	_ = agentsDirectory.Close()
	relativeTarget := filepath.Join("..", ".agents", "skills")
	if currentTarget, err := confinedRoot.Readlink(".claude/skills"); err == nil {
		if currentTarget == relativeTarget {
			return nil
		}
	} else if info, statErr := confinedRoot.Lstat(".claude/skills"); statErr == nil &&
		info.Mode()&os.ModeSymlink == 0 &&
		info.IsDir() &&
		!refreshMirror {
		return nil
	}
	// 后续操作必须继续使用已经固定的 owner 根 fd。这里不能在完成
	// owner 校验后重新按绝对路径打开 root，否则目录项被替换时会出现
	// TOCTOU 窗口。
	if err := ensureRelativeSymlinkAt(confinedRoot, ".claude/skills", relativeTarget); err == nil {
		return nil
	} else if mirrorErr := mirrorDirectoryAt(confinedRoot, ".agents/skills", ".claude/skills"); mirrorErr != nil {
		return fmt.Errorf("创建用户 Skill Claude 入口失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}

// ReplaceDirectory 原子替换一个 Skill 源目录，避免 runtime 读到半份文件。
func ReplaceDirectory(sourceRoot string, targetRoot string) error {
	return ReplaceDirectoryWithin(filepath.Dir(targetRoot), sourceRoot, targetRoot)
}

// ReplaceDirectoryWithin 在固定 owner/app 边界内原子替换目录。
func ReplaceDirectoryWithin(boundaryRoot string, sourceRoot string, targetRoot string) error {
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
	if runtimeIsolationEnforced() {
		sourceFS, openErr := root.OpenRootNoSymlink(sourceRelative)
		if openErr != nil {
			return openErr
		}
		normalizeErr := normalizeWorkspaceTree(sourceFS)
		sourceFS.Close()
		if normalizeErr != nil {
			return normalizeErr
		}
	}
	return replaceDirectoryWithinRoot(root, sourceRelative, targetRelative)
}

// ReplaceDirectoryAt 在已经固定的根目录句柄内原子替换目录。
//
// 调用方必须先完成 owner 或 app 边界校验；此函数不再通过绝对路径重新
// 打开边界，避免校验后目录项被替换造成越权。
func ReplaceDirectoryAt(
	root *confinedfs.Root,
	sourceRelative string,
	targetRelative string,
) error {
	return replaceDirectoryWithinRoot(
		root,
		filepath.ToSlash(sourceRelative),
		filepath.ToSlash(targetRelative),
	)
}

// RemoveEntryWithin 删除固定 owner/app 边界内的单个目录树。
func RemoveEntryWithin(boundaryRoot string, targetPath string) error {
	root, err := confinedfs.Open(boundaryRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	relativePath, err := relativePathWithin(boundaryRoot, targetPath)
	if err != nil {
		return err
	}
	return root.RemoveAll(relativePath)
}

func mirrorDirectory(boundaryRoot string, sourceRoot string, targetRoot string) error {
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
	return mirrorDirectoryAt(root, sourceRelative, targetRelative)
}

// mirrorDirectoryAt 在已经固定的 owner 根中复制 Skill 镜像。
func mirrorDirectoryAt(
	root *confinedfs.Root,
	sourceRelative string,
	targetRelative string,
) error {
	temporaryRelative, err := root.MkdirTemp(".settings/skill-staging", ".user-skill-mirror-", 0o700)
	if err != nil {
		return err
	}
	defer root.RemoveAll(temporaryRelative)
	sourceFS, err := root.OpenRootNoSymlink(sourceRelative)
	if err != nil {
		return err
	}
	temporaryFS, err := root.OpenRootNoSymlink(temporaryRelative)
	if err != nil {
		sourceFS.Close()
		return err
	}
	err = temporaryFS.CopyTreeFrom(sourceFS)
	sourceFS.Close()
	if err != nil {
		temporaryFS.Close()
		return err
	}
	if runtimeIsolationEnforced() {
		if err = normalizeWorkspaceTree(temporaryFS); err != nil {
			temporaryFS.Close()
			return err
		}
	}
	temporaryFS.Close()
	return replaceDirectoryWithinRoot(root, temporaryRelative, targetRelative)
}

func normalizeWorkspaceTree(root *confinedfs.Root) error {
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
			return confinedfs.ErrSymlink
		}
		if info.IsDir() {
			child, err := root.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return err
			}
			normalizeErr := normalizeWorkspaceTree(child)
			child.Close()
			if normalizeErr != nil {
				return normalizeErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return errors.New("workspace tree contains a special file")
		}
		file, err := root.OpenFileNoSymlink(entry.Name(), os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		openedInfo, err := file.Stat()
		if err == nil {
			err = file.Chmod(workspaceCopyFileMode(openedInfo.Mode()))
		}
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return root.ChmodRoot(workspaceDirectoryMode())
}
