// INPUT: Nexus 配置根、Agent workspace 根与迁移步骤。
// OUTPUT: 按顺序执行尚未完成的文件迁移，并原子写入全局完成标记。
// POS: 非数据库数据迁移的唯一账本；业务初始化器不保存迁移状态。
package migration

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	legacyMemorySessionsMigrationName  = "20260710_remove_legacy_memory_sessions"
	legacyMemoryDirectoryMigrationName = "20260710_remove_legacy_memory_directories"
	retiredManagedSkillsMigrationName  = "20260712_remove_retired_managed_skills"
	legacyMemoryManagerMigrationName   = "20260713_remove_legacy_memory_manager_skill"
)

type workspaceFileMigrationContext struct {
	configRoot    string
	workspaceRoot string
}

type workspaceFileMigration struct {
	name  string
	apply func(workspaceFileMigrationContext) (int, error)
}

var workspaceFileMigrations = []workspaceFileMigration{
	{name: legacyMemorySessionsMigrationName, apply: removeLegacyMemorySessions},
	{name: legacyMemoryDirectoryMigrationName, apply: removeLegacyMemoryDirectories},
	{name: retiredManagedSkillsMigrationName, apply: removeRetiredManagedSkills},
	{name: legacyMemoryManagerMigrationName, apply: removeLegacyMemoryManagerSkill},
}

// RunWorkspaceFiles 执行所有尚未完成的工作区文件迁移。
func RunWorkspaceFiles(configRoot string, workspaceRoot string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	migrationContext := workspaceFileMigrationContext{
		configRoot:    filepath.Clean(configRoot),
		workspaceRoot: filepath.Clean(workspaceRoot),
	}
	for _, migration := range workspaceFileMigrations {
		markerPath := workspaceFileMigrationMarker(migrationContext.configRoot, migration.name)
		applied, err := workspaceFileMigrationApplied(markerPath)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		affectedPaths, err := migration.apply(migrationContext)
		if err != nil {
			return fmt.Errorf("执行工作区文件迁移 %s: %w", migration.name, err)
		}
		if err = writeWorkspaceFileMigrationMarker(markerPath); err != nil {
			return err
		}
		logger.Info("工作区文件迁移完成",
			"migration", migration.name,
			"affected_paths", affectedPaths,
		)
	}
	return nil
}

func workspaceFileMigrationApplied(markerPath string) (bool, error) {
	content, err := os.ReadFile(markerPath)
	if err == nil {
		if string(content) != "completed\n" {
			return false, fmt.Errorf("工作区迁移标记内容无效 %q", markerPath)
		}
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("检查工作区迁移标记 %q: %w", markerPath, err)
	}
	return false, nil
}

func workspaceFileMigrationMarker(configRoot string, migrationName string) string {
	return filepath.Join(configRoot, ".migrations", migrationName)
}

// writeWorkspaceFileMigrationMarker 原子写入完成标记，避免半写入被误判为成功。
func writeWorkspaceFileMigrationMarker(markerPath string) error {
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o700); err != nil {
		return fmt.Errorf("创建工作区迁移标记目录: %w", err)
	}
	temporaryPath := markerPath + ".tmp"
	if err := os.WriteFile(temporaryPath, []byte("completed\n"), 0o600); err != nil {
		return fmt.Errorf("写入工作区迁移临时标记 %q: %w", temporaryPath, err)
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("收紧工作区迁移临时标记权限 %q: %w", temporaryPath, err)
	}
	if err := os.Rename(temporaryPath, markerPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("提交工作区迁移标记 %q: %w", markerPath, err)
	}
	return nil
}
