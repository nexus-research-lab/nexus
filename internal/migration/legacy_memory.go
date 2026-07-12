// INPUT: 历史 Agent/Room workspace 目录布局与旧记忆文件指纹。
// OUTPUT: 精确删除旧记忆产物；与新 SDK 共存的 topic、daily log 和索引保持不变。
// POS: 保留 20260710 两项升级迁移，禁止按同名目录猜测性删除用户数据。
package migration

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var legacyMemoryDiaryHeadingPattern = regexp.MustCompile(`^### \d{4}-\d{2}-\d{2} \d{2}:\d{2} - \[[A-Z]+\] .+$`)

func removeLegacyMemorySessions(migrationContext workspaceFileMigrationContext) (int, error) {
	directories, err := knownMemoryDirectories(migrationContext)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, memoryDirectory := range directories {
		removedPaths, cleanErr := cleanLegacyMemorySessions(filepath.Join(memoryDirectory, "sessions"))
		if cleanErr != nil {
			return removed, cleanErr
		}
		removed += removedPaths
		removedDirectory, removeErr := removeEmptyLegacyDirectory(memoryDirectory, removedPaths > 0)
		if removeErr != nil {
			return removed, removeErr
		}
		removed += removedDirectory
	}
	return removed, nil
}

func removeLegacyMemoryDirectories(migrationContext workspaceFileMigrationContext) (int, error) {
	directories, err := knownMemoryDirectories(migrationContext)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, directory := range directories {
		removedPaths, cleanErr := cleanLegacyMemoryDirectory(directory)
		if cleanErr != nil {
			return removed, cleanErr
		}
		removed += removedPaths
	}
	return removed, nil
}

func cleanLegacyMemorySessions(directory string) (int, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("读取旧记忆会话目录 %q: %w", directory, err)
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		legacy, detectErr := hasLegacySessionSummaryFingerprint(path)
		if detectErr != nil {
			return removed, detectErr
		}
		if !legacy {
			continue
		}
		if err = os.Remove(path); err != nil {
			return removed, fmt.Errorf("删除旧记忆会话文件 %q: %w", path, err)
		}
		removed++
	}
	removedDirectory, err := removeEmptyLegacyDirectory(directory, removed > 0)
	return removed + removedDirectory, err
}

func cleanLegacyMemoryDirectory(directory string) (int, error) {
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("检查旧记忆目录 %q: %w", directory, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, fmt.Errorf("读取旧记忆目录 %q: %w", directory, err)
	}
	removed := 0
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		switch {
		case entry.Name() == "checkpoints.json" && !entry.IsDir() && entry.Type()&os.ModeSymlink == 0:
			legacy, detectErr := hasLegacyCheckpointFingerprint(path)
			if detectErr != nil {
				return removed, detectErr
			}
			if !legacy {
				continue
			}
			if err = os.Remove(path); err != nil {
				return removed, fmt.Errorf("删除旧记忆检查点 %q: %w", path, err)
			}
			removed++
		case entry.Name() == "sessions" && entry.IsDir() && entry.Type()&os.ModeSymlink == 0:
			removedPaths, cleanErr := cleanLegacyMemorySessions(path)
			if cleanErr != nil {
				return removed, cleanErr
			}
			removed += removedPaths
		case !entry.IsDir() && entry.Type()&os.ModeSymlink == 0:
			legacy, detectErr := hasLegacyMemoryDiaryFingerprint(path)
			if detectErr != nil {
				return removed, detectErr
			}
			if !legacy {
				continue
			}
			if err = os.Remove(path); err != nil {
				return removed, fmt.Errorf("删除旧记忆日记 %q: %w", path, err)
			}
			removed++
		}
	}
	removedDirectory, err := removeEmptyLegacyDirectory(directory, removed > 0)
	return removed + removedDirectory, err
}

func hasLegacyCheckpointFingerprint(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("读取旧记忆检查点候选 %q: %w", path, err)
	}
	var document map[string]json.RawMessage
	if err = json.Unmarshal(content, &document); err != nil {
		return false, nil
	}
	scopes, exists := document["scopes"]
	if !exists {
		return false, nil
	}
	var values map[string]json.RawMessage
	return json.Unmarshal(scopes, &values) == nil, nil
}

func hasLegacySessionSummaryFingerprint(path string) (bool, error) {
	return scanMigrationFile(path, func(line string, state *legacyFileFingerprint) {
		switch {
		case strings.HasPrefix(line, "- Entry: "):
			state.hasEntry = true
		case strings.HasPrefix(line, "- Scope: "):
			state.hasScope = true
		}
	}, func(state legacyFileFingerprint) bool {
		return state.hasEntry && state.hasScope
	})
}

func hasLegacyMemoryDiaryFingerprint(path string) (bool, error) {
	if filepath.Ext(path) != ".md" {
		return false, nil
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))); err != nil {
		return false, nil
	}
	return scanMigrationFile(path, func(line string, state *legacyFileFingerprint) {
		if legacyMemoryDiaryHeadingPattern.MatchString(line) {
			state.hasHeading = true
		}
		if strings.HasPrefix(line, "*   **ID**: ") || strings.HasPrefix(line, "* **ID**: ") {
			state.hasEntry = true
		}
	}, func(state legacyFileFingerprint) bool {
		return state.hasHeading && state.hasEntry
	})
}

type legacyFileFingerprint struct {
	hasHeading bool
	hasEntry   bool
	hasScope   bool
}

func scanMigrationFile(
	path string,
	observe func(string, *legacyFileFingerprint),
	matches func(legacyFileFingerprint) bool,
) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("读取旧记忆候选文件 %q: %w", path, err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, 64<<10))
	if err != nil {
		return false, fmt.Errorf("扫描旧记忆候选文件 %q: %w", path, err)
	}
	state := legacyFileFingerprint{}
	for _, line := range strings.Split(string(content), "\n") {
		observe(line, &state)
		if matches(state) {
			return true, nil
		}
	}
	return false, nil
}

func removeEmptyLegacyDirectory(directory string, changed bool) (int, error) {
	if !changed {
		return 0, nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, fmt.Errorf("复查旧记忆目录 %q: %w", directory, err)
	}
	if len(entries) != 0 {
		return 0, nil
	}
	if err = os.Remove(directory); err != nil {
		return 0, fmt.Errorf("删除空旧记忆目录 %q: %w", directory, err)
	}
	return 1, nil
}

func workspaceMemoryDirectories(root string) ([]string, error) {
	directories, err := directChildMemoryDirectories(root)
	if err != nil {
		return nil, err
	}
	entries, err := readMigrationRoot(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "user_") {
			continue
		}
		nested, nestedErr := directChildMemoryDirectories(filepath.Join(root, entry.Name()))
		if nestedErr != nil {
			return nil, nestedErr
		}
		directories = append(directories, nested...)
	}
	return directories, nil
}

func knownMemoryDirectories(migrationContext workspaceFileMigrationContext) ([]string, error) {
	workspaceMemory, err := workspaceMemoryDirectories(migrationContext.workspaceRoot)
	if err != nil {
		return nil, err
	}
	roomMemory, err := directChildMemoryDirectories(filepath.Join(migrationContext.configRoot, "rooms"))
	if err != nil {
		return nil, err
	}
	return uniqueCleanPaths(append(workspaceMemory, roomMemory...)), nil
}

func directChildMemoryDirectories(root string) ([]string, error) {
	workspaces, err := directChildDirectories(root)
	if err != nil {
		return nil, err
	}
	directories := make([]string, 0, len(workspaces))
	for _, workspacePath := range workspaces {
		directories = append(directories, filepath.Join(workspacePath, "memory"))
	}
	return directories, nil
}

func directChildDirectories(root string) ([]string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || root == "" {
		return nil, nil
	}
	entries, err := readMigrationRoot(root)
	if err != nil {
		return nil, err
	}
	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			directories = append(directories, filepath.Join(root, entry.Name()))
		}
	}
	return directories, nil
}

func readMigrationRoot(root string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取工作区迁移目录 %q: %w", root, err)
	}
	return entries, nil
}
