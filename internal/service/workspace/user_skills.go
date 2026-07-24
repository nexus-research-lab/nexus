// INPUT: 用户身份与 Nexus workspace 配置。
// OUTPUT: owner 共享 Skill 源、Claude 兼容入口与原子目录替换能力。
// POS: 外部 Skill 导入与 runtime 目录装配的文件边界。
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
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

// SkillLibraryRoots 返回 runtime 需要读取的平台与用户 Skill 根。
func SkillLibraryRoots(cfg config.Config, ownerUserID string) []string {
	return []string{appfs.PlatformSkillRoot(), UserSkillLibraryRoot(cfg, ownerUserID)}
}

// EnsureUserSkillLibrary 确保用户外部 Skill 同时具备 nxs 与 Claude 发现入口。
func EnsureUserSkillLibrary(cfg config.Config, ownerUserID string) error {
	return syncUserSkillLibrary(cfg, ownerUserID, false)
}

// RefreshUserSkillLibrary 在 owner 源变化后刷新 Claude fallback 镜像。
func RefreshUserSkillLibrary(cfg config.Config, ownerUserID string) error {
	return syncUserSkillLibrary(cfg, ownerUserID, true)
}

func syncUserSkillLibrary(cfg config.Config, ownerUserID string, refreshMirror bool) error {
	root := UserSkillLibraryRoot(cfg, ownerUserID)
	lockValue, _ := userSkillLibraryLocks.LoadOrStore(root, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(root, workspaceDirectoryMode()); err != nil {
		return err
	}
	confinedRoot, err := confinedfs.Open(root)
	if err != nil {
		return err
	}
	defer confinedRoot.Close()
	if err = confinedRoot.MkdirAll(".agents/skills", workspaceDirectoryMode()); err != nil {
		return err
	}
	agentsRoot := filepath.Join(root, ".agents", "skills")
	claudeRoot := filepath.Join(root, ".claude", "skills")
	relativeTarget := filepath.Join("..", ".agents", "skills")
	if currentTarget, err := confinedRoot.Readlink(".claude/skills"); err == nil {
		if currentTarget == relativeTarget {
			return nil
		}
	} else if info, statErr := confinedRoot.Stat(".claude/skills"); statErr == nil && info.IsDir() && !refreshMirror {
		return nil
	}
	if err := ensureRelativeSymlink(root, claudeRoot, relativeTarget); err == nil {
		return nil
	} else if mirrorErr := mirrorDirectory(root, agentsRoot, claudeRoot); mirrorErr != nil {
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
	if runtimeIsolationEnforced() {
		if err := normalizeWorkspaceTree(sourceRoot); err != nil {
			return err
		}
	}
	return replaceDirectoryWithin(boundaryRoot, sourceRoot, targetRoot)
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
	temporaryRelative, err := root.MkdirTemp(".settings/skill-staging", ".user-skill-mirror-", 0o700)
	if err != nil {
		return err
	}
	temporaryRoot := filepath.Join(boundaryRoot, filepath.FromSlash(temporaryRelative))
	defer root.RemoveAll(temporaryRelative)
	if err = copyDirectoryTree(sourceRoot, temporaryRoot); err != nil {
		return err
	}
	if runtimeIsolationEnforced() {
		if err = normalizeWorkspaceTree(temporaryRoot); err != nil {
			return err
		}
	}
	return replaceDirectoryWithin(boundaryRoot, temporaryRoot, targetRoot)
}

func normalizeWorkspaceTree(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			return os.Chmod(path, workspaceDirectoryMode())
		}
		return os.Chmod(path, workspaceCopyFileMode(info.Mode()))
	})
}
