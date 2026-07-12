// INPUT: Agent/Room workspace 与已退役系统 Skill 名称。
// OUTPUT: 一次性移除现代与 legacy 两套部署入口。
// POS: Skill 生命周期迁移；不参与每次 workspace 初始化。
package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

var retiredManagedSkillNames = []string{
	"room-collaboration",
	"scheduled-task-manager",
}

func removeRetiredManagedSkills(migrationContext workspaceFileMigrationContext) (int, error) {
	workspaces, err := knownWorkspaceDirectories(migrationContext)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, workspacePath := range workspaces {
		for _, skillName := range retiredManagedSkillNames {
			entries := deployedSkillEntries(workspacePath, skillName)
			entryCount, countErr := existingPathCount(entries)
			if countErr != nil {
				return removed, countErr
			}
			if err = workspacesvc.UndeploySkill(workspacePath, skillName); err != nil {
				return removed, fmt.Errorf("移除已退役 Skill %s: %w", skillName, err)
			}
			removed += entryCount
		}
	}
	return removed, nil
}

func knownWorkspaceDirectories(migrationContext workspaceFileMigrationContext) ([]string, error) {
	workspaceRoot := migrationContext.workspaceRoot
	direct, err := directChildDirectories(workspaceRoot)
	if err != nil {
		return nil, err
	}
	all := append([]string(nil), direct...)
	for _, directory := range direct {
		if !strings.HasPrefix(filepath.Base(directory), "user_") {
			continue
		}
		nested, nestedErr := directChildDirectories(directory)
		if nestedErr != nil {
			return nil, nestedErr
		}
		all = append(all, nested...)
	}
	rooms, err := directChildDirectories(filepath.Join(migrationContext.configRoot, "rooms"))
	if err != nil {
		return nil, err
	}
	all = append(all, rooms...)
	return uniqueCleanPaths(all), nil
}

func deployedSkillEntries(workspacePath string, skillName string) []string {
	return []string{
		filepath.Join(workspacePath, ".agents", "skills", skillName),
		filepath.Join(workspacePath, ".claude", "skills", skillName),
	}
}

func existingPathCount(paths []string) (int, error) {
	count := 0
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			count++
		} else if !os.IsNotExist(err) {
			return count, fmt.Errorf("检查待迁移路径 %q: %w", path, err)
		}
	}
	return count, nil
}

func uniqueCleanPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		cleanPath := filepath.Clean(strings.TrimSpace(path))
		if cleanPath == "." || cleanPath == "" {
			continue
		}
		if _, exists := seen[cleanPath]; exists {
			continue
		}
		seen[cleanPath] = struct{}{}
		result = append(result, cleanPath)
	}
	return result
}
