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

	agentsRoot := filepath.Join(root, ".agents", "skills")
	if err := os.MkdirAll(agentsRoot, 0o755); err != nil {
		return err
	}
	claudeRoot := filepath.Join(root, ".claude", "skills")
	relativeTarget := filepath.Join("..", ".agents", "skills")
	if currentTarget, err := os.Readlink(claudeRoot); err == nil {
		if currentTarget == relativeTarget {
			return nil
		}
	} else if info, statErr := os.Stat(claudeRoot); statErr == nil && info.IsDir() && !refreshMirror {
		return nil
	}
	if err := ensureRelativeSymlink(claudeRoot, relativeTarget); err == nil {
		return nil
	} else if mirrorErr := mirrorDirectory(agentsRoot, claudeRoot); mirrorErr != nil {
		return fmt.Errorf("创建用户 Skill Claude 入口失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}

// ReplaceDirectory 原子替换一个 Skill 源目录，避免 runtime 读到半份文件。
func ReplaceDirectory(sourceRoot string, targetRoot string) error {
	return replaceDirectory(sourceRoot, targetRoot)
}

func mirrorDirectory(sourceRoot string, targetRoot string) error {
	if err := os.MkdirAll(filepath.Dir(targetRoot), 0o755); err != nil {
		return err
	}
	temporaryRoot, err := os.MkdirTemp(filepath.Dir(targetRoot), ".user-skill-mirror-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryRoot)
	if err = copyDirectoryTree(sourceRoot, temporaryRoot); err != nil {
		return err
	}
	return replaceDirectory(temporaryRoot, targetRoot)
}
