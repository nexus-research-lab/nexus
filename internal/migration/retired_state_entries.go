// INPUT: 宿主 app 根与其所在状态根。
// OUTPUT: 删除已无读写方的状态根残留：孤立 session-memory project、app 内旧记忆入口、
// 空调试目录、零字节旧库、根级旧 runtime 参数目录与非 Linux 的空共享工作区根。
// POS: 一次性尽力清理；只删除可识别为残留的条目，失败不阻断启动，下次启动重试。
package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// 旧 SDK 以摘要目录作 cwd 运行 session-memory fork，project 名因此形如
// <sanitized runtime/projects/...>-<session>-session-memory，内容只有 fork transcript。
// 已从代码中删除的 2026-10 之前迁移的完成标记；迁移本身不再存在，标记只是噪音。
var retiredMigrationMarkers = []string{
	"20260710_remove_legacy_memory_sessions",
	"20260710_remove_legacy_memory_directories",
	"20260712_remove_retired_managed_skills",
	"20260713_remove_legacy_memory_manager_skill",
	"20260723_migrate_v0_1_27_skill_storage",
	"20260723_state_layout_v1",
	"20260723_workspace_layout_v1",
	"20260727_move_room_state_to_user_state",
	"20260727_consolidate_legacy_no_user_input_conversations",
}

const (
	orphanSessionMemoryProjectMarker = "-runtime-projects-"
	orphanSessionMemoryProjectSuffix = "-session-memory"
)

func removeRetiredStateEntries(migrationContext workspaceFileMigrationContext) (int, error) {
	appRoot := migrationContext.configRoot
	stateRoot := filepath.Dir(appRoot)
	removed := 0
	var errs []error
	collect := func(count int, err error) {
		removed += count
		if err != nil {
			errs = append(errs, err)
		}
	}

	owners, err := directChildDirectories(filepath.Join(stateRoot, "users"))
	if err != nil {
		errs = append(errs, err)
	}
	for _, owner := range owners {
		collect(removeOrphanSessionMemoryProjects(filepath.Join(owner, "runtime", "projects")))
	}

	for _, name := range retiredMigrationMarkers {
		collect(removeRegularFile(workspaceFileMigrationMarker(appRoot, name), false))
	}
	// app 是宿主控制面，记忆只属于 Agent workspace；旧 runtime 误把 app 当 cwd 时留下这对入口。
	collect(removeEmptyDirectory(filepath.Join(appRoot, "memory")))
	if _, statErr := os.Lstat(filepath.Join(appRoot, "memory")); os.IsNotExist(statErr) {
		collect(removeRegularFile(filepath.Join(appRoot, "MEMORY.md"), false))
	}
	// 桌面壳曾预建 app/debug，但没有任何进程写入。
	collect(removeEmptyDirectory(filepath.Join(appRoot, "debug")))
	// 数据库早已固定为 nexus.db；零字节 nexus.sqlite 是更早版本的占位。
	collect(removeRegularFile(filepath.Join(appRoot, "data", "nexus.sqlite"), true))
	// runtime 参数文件现在位于 users/<owner>/runtime/runtime/arg-files。
	legacyRuntimeRoot := filepath.Join(stateRoot, "runtime")
	if ok, checkErr := isRealDirectory(filepath.Join(legacyRuntimeRoot, "arg-files")); checkErr != nil {
		errs = append(errs, checkErr)
	} else if ok {
		if err := removeRetiredTree(filepath.Join(legacyRuntimeRoot, "arg-files")); err != nil {
			errs = append(errs, err)
		} else {
			removed++
		}
	}
	collect(removeEmptyDirectory(legacyRuntimeRoot))
	// 共享工作区只在 Linux 强隔离下使用；其他平台的空目录由旧布局迁移预建。
	if runtime.GOOS != "linux" {
		collect(removeEmptyDirectory(filepath.Join(stateRoot, "shared-workspaces")))
	}
	return removed, errors.Join(errs...)
}

func removeOrphanSessionMemoryProjects(projectsRoot string) (int, error) {
	ok, err := isRealDirectory(projectsRoot)
	if err != nil || !ok {
		return 0, err
	}
	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		return 0, fmt.Errorf("读取 runtime projects 目录 %q: %w", projectsRoot, err)
	}
	removed := 0
	var errs []error
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.Contains(name, orphanSessionMemoryProjectMarker) ||
			!strings.HasSuffix(name, orphanSessionMemoryProjectSuffix) {
			continue
		}
		if err := removeRetiredTree(filepath.Join(projectsRoot, name)); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}
