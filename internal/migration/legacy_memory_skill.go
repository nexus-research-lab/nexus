// INPUT: Agent/Room workspace 与旧版 Nexus memory-manager 部署。
// OUTPUT: 精确识别并移除现代与 legacy 两套内置 Skill 入口。
// POS: 记忆能力迁入 nxs 后的一次性生命周期迁移；保留用户自建的同名 Skill。
package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

const legacyMemoryManagerSkillName = "memory-manager"

var legacyMemoryManagerMarkers = []string{
	"name: memory-manager",
	"# memory-manager",
	"真正的记忆能力已经沉到 `nexusctl memory`",
}

func removeLegacyMemoryManagerSkill(migrationContext workspaceFileMigrationContext) (int, error) {
	workspaces, err := knownWorkspaceDirectories(migrationContext)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, workspacePath := range workspaces {
		managed, detectErr := isLegacyMemoryManagerDeployment(workspacePath)
		if detectErr != nil {
			return removed, detectErr
		}
		if !managed {
			continue
		}

		entries := deployedSkillEntries(workspacePath, legacyMemoryManagerSkillName)
		entryCount, countErr := existingPathCount(entries)
		if countErr != nil {
			return removed, countErr
		}
		if err = workspacesvc.UndeploySkill(workspacePath, legacyMemoryManagerSkillName); err != nil {
			return removed, fmt.Errorf("移除旧版记忆 Skill: %w", err)
		}
		removed += entryCount
	}
	return removed, nil
}

func isLegacyMemoryManagerDeployment(workspacePath string) (bool, error) {
	entries := deployedSkillEntries(workspacePath, legacyMemoryManagerSkillName)
	legacyContentFound := false
	customContentFound := false
	for _, entryPath := range entries {
		content, err := os.ReadFile(filepath.Join(entryPath, "SKILL.md"))
		if err == nil {
			if containsAllMarkers(string(content), legacyMemoryManagerMarkers) {
				legacyContentFound = true
			} else {
				customContentFound = true
			}
			continue
		}
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("读取待迁移 Skill %q: %w", entryPath, err)
		}
	}

	// 两套入口内容不一致时按用户内容处理，避免误删被本地改写的同名 Skill。
	if customContentFound {
		return false, nil
	}
	if legacyContentFound {
		return true, nil
	}
	return isDanglingManagedSkillSymlink(entries)
}

func containsAllMarkers(content string, markers []string) bool {
	for _, marker := range markers {
		if !strings.Contains(content, marker) {
			return false
		}
	}
	return true
}

func isDanglingManagedSkillSymlink(entries []string) (bool, error) {
	if len(entries) != 2 {
		return false, nil
	}
	if _, err := os.Lstat(entries[0]); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("检查现代 Skill 入口 %q: %w", entries[0], err)
	}

	info, err := os.Lstat(entries[1])
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查 legacy Skill 入口 %q: %w", entries[1], err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := os.Readlink(entries[1])
	if err != nil {
		return false, fmt.Errorf("读取 legacy Skill 软链接 %q: %w", entries[1], err)
	}
	expected := filepath.Join("..", "..", ".agents", "skills", legacyMemoryManagerSkillName)
	return filepath.Clean(target) == expected, nil
}
