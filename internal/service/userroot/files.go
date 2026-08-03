// INPUT: 已授权的新旧 users 根与数据库中的 Agent 路径投影。
// OUTPUT: 不跟随链接的整树复制、目标目录准备与 canonical 路径校验。
// POS: users 根迁移的文件系统边界；所有绝对路径在这里收敛为固定根下的相对路径。
package userroot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type agentWorkspaceRecord struct {
	agentID          string
	ownerUserID      string
	workspacePath    string
	workspaceDirName string
}

func readAgentWorkspaceRecords(ctx context.Context, db *sql.DB) ([]agentWorkspaceRecord, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, COALESCE(owner_user_id, ''), workspace_path
FROM agents
ORDER BY owner_user_id ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 Agent workspace 路径: %w", err)
	}
	defer rows.Close()
	records := make([]agentWorkspaceRecord, 0)
	for rows.Next() {
		var record agentWorkspaceRecord
		if err = rows.Scan(&record.agentID, &record.ownerUserID, &record.workspacePath); err != nil {
			return nil, fmt.Errorf("扫描 Agent workspace 路径: %w", err)
		}
		record.agentID = strings.TrimSpace(record.agentID)
		record.ownerUserID = strings.TrimSpace(record.ownerUserID)
		if record.ownerUserID == "" {
			record.ownerUserID = "__system__"
		}
		record.workspacePath = strings.TrimSpace(record.workspacePath)
		record.workspaceDirName = filepath.Base(filepath.Clean(record.workspacePath))
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Agent workspace 路径: %w", err)
	}
	return records, nil
}

func validateAgentWorkspaceRecords(records []agentWorkspaceRecord, sourceRoot string, targetRoot string) error {
	for _, record := range records {
		targetPath := resolveAgentPathAt(targetRoot, record)
		if samePath(record.workspacePath, targetPath) {
			continue
		}
		sourcePath := resolveAgentPathAt(sourceRoot, record)
		if !samePath(record.workspacePath, sourcePath) {
			return fmt.Errorf(
				"Agent %s 的 workspace 路径不属于当前生效根",
				record.agentID,
			)
		}
	}
	return nil
}

func resolveAgentPathAt(root string, record agentWorkspaceRecord) string {
	directoryName := strings.TrimSpace(record.workspaceDirName)
	if directoryName == "" {
		directoryName = agentsvc.BuildWorkspaceDirName(record.agentID)
	}
	return filepath.Join(
		agentsvc.UserWorkspaceBasePath(config.Config{WorkspacePath: root}, record.ownerUserID),
		directoryName,
	)
}

func validateWorkspaceRoots(sourceRoot string, targetRoot string) error {
	sourceRoot = filepath.Clean(strings.TrimSpace(sourceRoot))
	targetRoot = filepath.Clean(strings.TrimSpace(targetRoot))
	if !filepath.IsAbs(sourceRoot) || !filepath.IsAbs(targetRoot) {
		return errors.New("users 根目录必须使用绝对路径")
	}
	if err := validateUsersRootTarget(targetRoot); err != nil {
		return err
	}
	if samePath(sourceRoot, targetRoot) {
		return nil
	}
	if pathContains(sourceRoot, targetRoot) || pathContains(targetRoot, sourceRoot) {
		return errors.New("新旧 users 根目录不能互相嵌套")
	}
	return nil
}

func validateUsersRootTarget(targetRoot string) error {
	appRoot := appfs.AppDir()
	if samePath(targetRoot, appRoot) ||
		pathContains(appRoot, targetRoot) ||
		pathContains(targetRoot, appRoot) {
		return errors.New("users 根与宿主 app 数据目录不能重叠")
	}
	return nil
}

func pathContains(parent string, child string) bool {
	relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil || relative == "." || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func requireMergeableTarget(targetRoot string, allowExisting bool) error {
	info, err := os.Lstat(targetRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检查新 users 根目录: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("新 users 根必须是真实目录")
	}
	entries, err := os.ReadDir(targetRoot)
	if err != nil {
		return fmt.Errorf("读取新 users 根目录: %w", err)
	}
	if len(entries) > 0 && !allowExisting {
		return errors.New("新 users 根目录必须为空，避免覆盖已有文件")
	}
	return nil
}

func copyUsersTree(ctx context.Context, sourceRoot string, targetRoot string) error {
	if samePath(sourceRoot, targetRoot) {
		return nil
	}
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return err
	}
	source, err := confinedfs.Open(sourceRoot)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := confinedfs.Open(targetRoot)
	if err != nil {
		return err
	}
	defer target.Close()
	return copyUsersDirectory(ctx, source, target, true)
}

func copyUsersDirectory(
	ctx context.Context,
	source *confinedfs.Root,
	target *confinedfs.Root,
	prune bool,
) error {
	sourceInfo, err := source.Stat(".")
	if err != nil {
		return err
	}
	entries, err := fs.ReadDir(source.FS(), ".")
	if err != nil {
		return err
	}
	if prune {
		if err = pruneTargetDirectory(target, entries); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}
		info, statErr := source.Lstat(entry.Name())
		if statErr != nil {
			return statErr
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			if err = copyUsersSymlink(source, target, entry.Name(), info); err != nil {
				return err
			}
		case info.IsDir():
			if err = prepareTargetDirectory(target, entry.Name()); err != nil {
				return err
			}
			sourceChild, openErr := source.OpenRootNoSymlink(entry.Name())
			if openErr != nil {
				return openErr
			}
			targetChild, targetErr := target.OpenOrCreateRootNoSymlink(entry.Name(), info.Mode().Perm())
			if targetErr != nil {
				sourceChild.Close()
				return targetErr
			}
			copyErr := copyUsersDirectory(ctx, sourceChild, targetChild, prune)
			sourceChild.Close()
			targetChild.Close()
			if copyErr != nil {
				return copyErr
			}
		case info.Mode().IsRegular():
			if err = prepareTargetFile(target, entry.Name()); err != nil {
				return err
			}
			if err = copyUsersFile(source, target, entry.Name(), info); err != nil {
				return err
			}
		case info.Mode()&(os.ModeSocket|os.ModeNamedPipe) != 0:
			// runtime 的 socket/FIFO 是进程期 IPC，不属于可恢复的用户持久数据。
			if err = target.RemoveAll(entry.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		default:
			return fmt.Errorf("users 数据包含无法迁移的特殊文件: %s", entry.Name())
		}
	}
	return target.ChmodRoot(sourceInfo.Mode().Perm())
}

func pruneTargetDirectory(target *confinedfs.Root, sourceEntries []fs.DirEntry) error {
	sourceNames := make(map[string]struct{}, len(sourceEntries))
	for _, entry := range sourceEntries {
		sourceNames[entry.Name()] = struct{}{}
	}
	targetEntries, err := fs.ReadDir(target.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range targetEntries {
		if _, exists := sourceNames[entry.Name()]; exists {
			continue
		}
		// 先清理再复制，使大小写不敏感卷上的 Foo -> foo 重命名也能稳定镜像。
		if err = target.RemoveAll(entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func prepareTargetDirectory(target *confinedfs.Root, name string) error {
	info, err := target.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	return target.RemoveAll(name)
}

func prepareTargetFile(target *confinedfs.Root, name string) error {
	info, err := target.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return nil
	}
	return target.RemoveAll(name)
}

func copyUsersFile(
	source *confinedfs.Root,
	target *confinedfs.Root,
	name string,
	expected os.FileInfo,
) error {
	file, err := source.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	observed, err := source.Lstat(name)
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || !observed.Mode().IsRegular() ||
		!os.SameFile(expected, opened) || !os.SameFile(expected, observed) {
		return confinedfs.ErrChanged
	}
	return target.WriteFileAtomicFrom(name, file, opened.Mode().Perm())
}

func copyUsersSymlink(
	source *confinedfs.Root,
	target *confinedfs.Root,
	name string,
	expected os.FileInfo,
) error {
	linkTarget, err := source.Readlink(name)
	if err != nil {
		return err
	}
	observed, err := source.Lstat(name)
	if err != nil {
		return err
	}
	if observed.Mode()&os.ModeSymlink == 0 || !os.SameFile(expected, observed) {
		return confinedfs.ErrChanged
	}
	if err = target.RemoveAll(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return target.Symlink(linkTarget, name)
}

func ensureTargetAgentDirectories(targetRoot string, records []agentWorkspaceRecord) error {
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return err
	}
	root, err := confinedfs.Open(targetRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, record := range records {
		targetPath := resolveAgentPathAt(targetRoot, record)
		relative, relErr := filepath.Rel(targetRoot, targetPath)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("目标 Agent workspace 越过新根")
		}
		directory, openErr := root.OpenOrCreateRootNoSymlink(filepath.ToSlash(relative), 0o770)
		if openErr != nil {
			return openErr
		}
		if closeErr := directory.Close(); closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func rebaseTranscriptProjects(
	ctx context.Context,
	sourceRoot string,
	targetRoot string,
	records []agentWorkspaceRecord,
) error {
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		projectsRoot := filepath.Join(
			appfs.UserRuntimeRootAtUsersRoot(targetRoot, record.ownerUserID),
			"projects",
		)
		root, err := confinedfs.Open(projectsRoot)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		oldWorkspacePath := resolveAgentPathAt(sourceRoot, record)
		newWorkspacePath := resolveAgentPathAt(targetRoot, record)
		newProjectName := workspacestore.TranscriptProjectDirectoryName(newWorkspacePath)
		for _, oldProjectName := range workspacestore.TranscriptProjectDirectoryNames(oldWorkspacePath) {
			if oldProjectName == newProjectName {
				continue
			}
			if err = mergeTranscriptProject(ctx, root, oldProjectName, newProjectName); err != nil {
				root.Close()
				return fmt.Errorf("迁移 Agent %s transcript 项目目录: %w", record.agentID, err)
			}
		}
		if err = root.Close(); err != nil {
			return err
		}
	}
	return nil
}

func mergeTranscriptProject(
	ctx context.Context,
	projectsRoot *confinedfs.Root,
	sourceName string,
	targetName string,
) error {
	sourceInfo, err := projectsRoot.Lstat(sourceName)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() || sourceInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("旧 transcript 项目不是安全目录")
	}
	source, err := projectsRoot.OpenRootNoSymlink(sourceName)
	if err != nil {
		return err
	}
	target, err := projectsRoot.OpenOrCreateRootNoSymlink(targetName, sourceInfo.Mode().Perm())
	if err != nil {
		source.Close()
		return err
	}
	copyErr := copyUsersDirectory(ctx, source, target, false)
	source.Close()
	target.Close()
	if copyErr != nil {
		return copyErr
	}
	return projectsRoot.RemoveAll(sourceName)
}
