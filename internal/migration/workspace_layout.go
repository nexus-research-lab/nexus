// INPUT: 已完成数据库 schema 迁移的数据库、旧 workspace 根和 Nexus 状态根。
// OUTPUT: 按 owner 重排 workspace，并同步更新 agents.workspace_path。
// POS: 状态根迁移后的第二阶段文件迁移；文件与数据库均成功后才写完成标记。
package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const workspaceLayoutMigrationName = "20260723_workspace_layout_v1"

type workspaceLayoutAgent struct {
	id                       string
	ownerUserID              string
	persistedOwnerUserID     string
	workspacePath            string
	legacyTranscriptProjects []string
}

// RunWorkspaceLayout 将旧 workspace/<owner>/<agent> 迁移到 users/<owner>/workspace。
func RunWorkspaceLayout(
	ctx context.Context,
	cfg config.Config,
	stateRoot string,
	logger *slog.Logger,
) error {
	return runWorkspaceLayout(
		ctx,
		cfg,
		stateRoot,
		logger,
		appfs.RuntimeIsolationEnforced(),
	)
}

func runWorkspaceLayout(
	ctx context.Context,
	cfg config.Config,
	stateRoot string,
	logger *slog.Logger,
	launcherManagesPermissions bool,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	stateRoot = filepath.Clean(stateRoot)
	appRoot := filepath.Join(stateRoot, "app")
	markerPath := workspaceFileMigrationMarker(appRoot, workspaceLayoutMigrationName)
	applied, err := workspaceFileMigrationApplied(markerPath)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("打开 workspace 布局迁移数据库: %w", err)
	}
	defer db.Close()

	agents, err := loadWorkspaceLayoutAgents(ctx, db)
	if err != nil {
		return err
	}
	owners, err := loadWorkspaceLayoutOwners(ctx, db)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		owners[normalizeWorkspaceOwner(agent.ownerUserID)] = struct{}{}
	}
	owners[authctx.SystemUserID] = struct{}{}
	if err = validateWorkspaceOwnerSegments(stateRoot, owners); err != nil {
		return err
	}
	if err = validateLegacyWorkspaceOwnerSources(owners); err != nil {
		return err
	}
	if err = validateWorkspaceLayoutAgents(stateRoot, agents); err != nil {
		return err
	}
	captureLegacyTranscriptProjectNames(stateRoot, agents)

	legacyRoot := filepath.Join(stateRoot, "workspace")
	if err = migrateOwnerWorkspaceDirectories(legacyRoot, stateRoot, owners); err != nil {
		return err
	}
	if err = migrateSystemWorkspaceRemainder(legacyRoot, stateRoot); err != nil {
		return err
	}
	migratedProjects, err := migrateOwnerTranscriptProjects(stateRoot, agents)
	if err != nil {
		return err
	}

	updates := make([]workspaceLayoutAgent, 0)
	for _, agent := range agents {
		targetPath := agent.workspacePath
		if mappedPath, ok := targetWorkspacePath(
			stateRoot,
			agent.ownerUserID,
			agent.workspacePath,
		); ok {
			targetPath = mappedPath
		}
		ownerChanged := agent.persistedOwnerUserID != agent.ownerUserID
		pathChanged := filepath.Clean(targetPath) != filepath.Clean(agent.workspacePath)
		if !ownerChanged && !pathChanged {
			continue
		}
		updates = append(updates, workspaceLayoutAgent{
			id:            agent.id,
			ownerUserID:   agent.ownerUserID,
			workspacePath: targetPath,
		})
	}
	if err = updateWorkspaceLayoutAgentPaths(ctx, db, cfg.DatabaseDriver, updates); err != nil {
		return err
	}
	if err = hardenMigratedWorkspaceLayout(
		filepath.Join(stateRoot, "users"),
		launcherManagesPermissions,
	); err != nil {
		return fmt.Errorf("收紧 workspace 用户目录权限: %w", err)
	}
	if err = writeWorkspaceFileMigrationMarker(markerPath); err != nil {
		return err
	}
	logger.Info("workspace 布局迁移完成",
		"migration", workspaceLayoutMigrationName,
		"state_root", stateRoot,
		"updated_agents", len(updates),
		"migrated_transcript_projects", migratedProjects,
	)
	return nil
}

func hardenMigratedWorkspaceLayout(
	usersRoot string,
	launcherManagesPermissions bool,
) error {
	if !shouldHardenMigratedPermissions(launcherManagesPermissions) {
		// 桌面端保留宿主用户的原生权限；Linux enforce 由 launcher 的
		// identity sync 恢复 owner、私有组和默认 ACL。
		return nil
	}
	return hardenLayoutTree(usersRoot)
}

func validateWorkspaceOwnerSegments(stateRoot string, owners map[string]struct{}) error {
	segments := make(map[string]string, len(owners))
	for ownerUserID := range owners {
		segment := appfs.UserPathSegment(ownerUserID)
		segmentKey := layoutComparisonPath(segment)
		if previousOwner, exists := segments[segmentKey]; exists && previousOwner != ownerUserID {
			return fmt.Errorf(
				"owner 用户目录发生路径碰撞: %q 与 %q 都映射到 %q",
				previousOwner,
				ownerUserID,
				filepath.Join(filepath.Clean(stateRoot), "users", segment),
			)
		}
		segments[segmentKey] = ownerUserID
	}
	return nil
}

func validateLegacyWorkspaceOwnerSources(owners map[string]struct{}) error {
	type ownerSource struct {
		ownerUserID string
		sourceName  string
	}
	sources := make([]ownerSource, 0, len(owners)*3)
	for ownerUserID := range owners {
		if ownerUserID == authctx.SystemUserID {
			continue
		}
		for _, sourceName := range workspaceOwnerSourceNames("", ownerUserID) {
			sources = append(sources, ownerSource{
				ownerUserID: ownerUserID,
				sourceName:  filepath.Clean(sourceName),
			})
		}
	}
	for leftIndex := range sources {
		for rightIndex := leftIndex + 1; rightIndex < len(sources); rightIndex++ {
			left := sources[leftIndex]
			right := sources[rightIndex]
			if left.ownerUserID == right.ownerUserID ||
				!layoutPathsOverlap(left.sourceName, right.sourceName) {
				continue
			}
			return fmt.Errorf(
				"旧 workspace owner 路径发生交叉: owner=%q path=%q owner=%q path=%q",
				left.ownerUserID,
				left.sourceName,
				right.ownerUserID,
				right.sourceName,
			)
		}
	}
	return nil
}

func layoutPathsOverlap(left string, right string) bool {
	left = layoutComparisonPath(left)
	right = layoutComparisonPath(right)
	if left == right {
		return true
	}
	separator := string(os.PathSeparator)
	return strings.HasPrefix(left, right+separator) ||
		strings.HasPrefix(right, left+separator)
}

func layoutComparisonPath(path string) string {
	clean := filepath.Clean(path)
	if os.PathSeparator == '\\' {
		return strings.ToLower(clean)
	}
	return clean
}

func validateWorkspaceLayoutAgents(stateRoot string, agents []workspaceLayoutAgent) error {
	usersRoot := filepath.Clean(filepath.Join(stateRoot, "users"))
	for _, agent := range agents {
		workspacePath := normalizeMigrationPath(agent.workspacePath, stateRoot)
		if workspacePath == "" {
			continue
		}
		relative, err := filepath.Rel(usersRoot, workspacePath)
		if err != nil || relative == "." || relative == "" ||
			relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			continue
		}
		parts := strings.Split(filepath.Clean(relative), string(os.PathSeparator))
		if len(parts) < 2 || parts[1] != "workspace" {
			continue
		}
		expectedSegment := appfs.UserPathSegment(agent.ownerUserID)
		if parts[0] != expectedSegment {
			return fmt.Errorf(
				"Agent %s 的 workspace 属于其他 owner 目录: path=%q owner=%q",
				agent.id,
				agent.workspacePath,
				agent.ownerUserID,
			)
		}
	}
	return nil
}

func loadWorkspaceLayoutAgents(ctx context.Context, db *sql.DB) ([]workspaceLayoutAgent, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, COALESCE(owner_user_id, ''), workspace_path
FROM agents
ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 workspace Agent 记录: %w", err)
	}
	defer rows.Close()

	agents := make([]workspaceLayoutAgent, 0)
	for rows.Next() {
		var agent workspaceLayoutAgent
		if err = rows.Scan(&agent.id, &agent.persistedOwnerUserID, &agent.workspacePath); err != nil {
			return nil, fmt.Errorf("扫描 workspace Agent 记录: %w", err)
		}
		agent.persistedOwnerUserID = strings.TrimSpace(agent.persistedOwnerUserID)
		agent.ownerUserID = normalizeWorkspaceOwner(agent.persistedOwnerUserID)
		agent.workspacePath = strings.TrimSpace(agent.workspacePath)
		agents = append(agents, agent)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 workspace Agent 记录: %w", err)
	}
	return agents, nil
}

func loadWorkspaceLayoutOwners(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT owner_user_id FROM owner_profiles ORDER BY owner_user_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 workspace 用户记录: %w", err)
	}
	defer rows.Close()

	owners := make(map[string]struct{})
	for rows.Next() {
		var ownerUserID string
		if err = rows.Scan(&ownerUserID); err != nil {
			return nil, fmt.Errorf("扫描 workspace 用户记录: %w", err)
		}
		owners[normalizeWorkspaceOwner(ownerUserID)] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 workspace 用户记录: %w", err)
	}
	return owners, nil
}

func migrateOwnerWorkspaceDirectories(
	legacyRoot string,
	stateRoot string,
	owners map[string]struct{},
) error {
	ownerIDs := make([]string, 0, len(owners))
	for ownerUserID := range owners {
		ownerIDs = append(ownerIDs, ownerUserID)
	}
	sort.Strings(ownerIDs)
	for _, ownerUserID := range ownerIDs {
		if ownerUserID == authctx.SystemUserID {
			continue
		}
		targetRoot := appfs.UserWorkspaceRootAt(stateRoot, ownerUserID)
		for _, sourceName := range workspaceOwnerSourceNames(stateRoot, ownerUserID) {
			sourcePath := filepath.Join(legacyRoot, sourceName)
			if _, err := os.Lstat(sourcePath); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				return fmt.Errorf("读取 owner %s workspace: %w", ownerUserID, err)
			}
			if _, err := moveLayoutEntry(sourcePath, targetRoot); err != nil {
				return fmt.Errorf("迁移 owner %s workspace: %w", ownerUserID, err)
			}
		}
	}
	return nil
}

func migrateSystemWorkspaceRemainder(legacyRoot string, stateRoot string) error {
	entries, err := os.ReadDir(legacyRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取系统 workspace 根: %w", err)
	}
	targetRoot := appfs.UserWorkspaceRootAt(stateRoot, authctx.SystemUserID)
	for _, entry := range entries {
		sourcePath := filepath.Join(legacyRoot, entry.Name())
		targetPath := filepath.Join(targetRoot, entry.Name())
		if _, moveErr := moveLayoutEntry(sourcePath, targetPath); moveErr != nil {
			return fmt.Errorf("迁移系统 workspace 条目 %q: %w", entry.Name(), moveErr)
		}
	}
	remaining, err := os.ReadDir(legacyRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检查系统 workspace 根: %w", err)
	}
	if len(remaining) == 0 {
		if err = os.Remove(legacyRoot); err != nil {
			return fmt.Errorf("删除空旧 workspace 根: %w", err)
		}
	}
	return nil
}

// migrateOwnerTranscriptProjects 将旧的全局 transcript 项目目录按 Agent owner
// 分发到用户 runtime。只迁移能由数据库 Agent workspace 精确推导出的目录；
// 未能归属的目录继续留在系统 runtime，避免把数据错误授予某个用户。
func migrateOwnerTranscriptProjects(
	stateRoot string,
	agents []workspaceLayoutAgent,
) (int, error) {
	systemProjectsRoot := filepath.Join(
		appfs.UserRuntimeRootAt(stateRoot, authctx.SystemUserID),
		"projects",
	)
	affected := 0
	for _, agent := range agents {
		ownerUserID := normalizeWorkspaceOwner(agent.ownerUserID)
		workspacePath := normalizeMigrationPath(agent.workspacePath, stateRoot)
		if workspacePath == "" {
			continue
		}
		sourceProjectNames := agent.legacyTranscriptProjects
		if len(sourceProjectNames) == 0 {
			sourceProjectNames = workspacepkg.TranscriptProjectDirectoryNames(workspacePath)
		}
		if len(sourceProjectNames) == 0 {
			continue
		}
		targetWorkspace, mapped := targetWorkspacePath(stateRoot, ownerUserID, agent.workspacePath)
		if !mapped {
			targetWorkspace = workspacePath
		}
		targetProjectName := workspacepkg.TranscriptProjectDirectoryName(targetWorkspace)
		if targetProjectName == "" {
			targetProjectName = sourceProjectNames[0]
		}
		targetProjectsRoot := appfs.UserRuntimeRootAt(stateRoot, ownerUserID)
		if !mapped {
			// 自定义 workspace 不带有 owner 目录语义，保留在系统 runtime；
			// 这样旧版全局 transcript 仍能被兼容回退路径读取，不猜测归属。
			targetProjectsRoot = appfs.UserRuntimeRootAt(stateRoot, authctx.SystemUserID)
		}
		for _, sourceProjectName := range sourceProjectNames {
			sourcePath := filepath.Join(systemProjectsRoot, sourceProjectName)
			if _, statErr := os.Lstat(sourcePath); errors.Is(statErr, os.ErrNotExist) {
				continue
			} else if statErr != nil {
				return affected, fmt.Errorf("读取 transcript 项目 %q: %w", sourceProjectName, statErr)
			}
			targetPath := filepath.Join(
				targetProjectsRoot,
				"projects",
				targetProjectName,
			)
			moved, moveErr := moveLayoutEntry(sourcePath, targetPath)
			if moveErr != nil {
				return affected, fmt.Errorf(
					"迁移 owner %s 的 transcript 项目 %q: %w",
					ownerUserID,
					sourceProjectName,
					moveErr,
				)
			}
			if moved {
				affected++
			}
		}
	}
	return affected, nil
}

func captureLegacyTranscriptProjectNames(
	stateRoot string,
	agents []workspaceLayoutAgent,
) {
	for index := range agents {
		workspacePath := normalizeMigrationPath(agents[index].workspacePath, stateRoot)
		if workspacePath == "" {
			continue
		}
		agents[index].legacyTranscriptProjects =
			workspacepkg.TranscriptProjectDirectoryNames(workspacePath)
	}
}

func workspaceOwnerSourceNames(stateRoot string, ownerUserID string) []string {
	ownerSegment := filepath.Base(appfs.UserDataRootAt(stateRoot, ownerUserID))
	names := make([]string, 0, 3)
	appendName := func(name string) {
		name = filepath.Clean(name)
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}
	appendName(agentpkg.BuildWorkspaceDirName(ownerUserID))
	appendName(ownerSegment)
	if rawOwner := strings.TrimSpace(ownerUserID); rawOwner != "" &&
		isSafeLegacyOwnerPath(rawOwner) {
		appendName(rawOwner)
	}
	return names
}

// isSafeLegacyOwnerPath 只允许把历史 owner 目录当作 workspace 根下的相对路径。
//
// 旧版本通常把 owner_user_id 原样拼进路径；迁移时仍兼容其中的斜杠，
// 但拒绝绝对路径和 ..，避免数据库中的异常值把迁移操作带出旧 workspace 根。
func isSafeLegacyOwnerPath(ownerUserID string) bool {
	if filepath.IsAbs(ownerUserID) {
		return false
	}
	clean := filepath.Clean(ownerUserID)
	if clean == "." || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return false
	}
	return clean == ownerUserID
}

func stripLegacyOwnerPrefix(relative string, stateRoot string, ownerUserID string) string {
	relative = filepath.Clean(relative)
	for _, sourceName := range workspaceOwnerSourceNames(stateRoot, ownerUserID) {
		sourceName = filepath.Clean(sourceName)
		if relative == sourceName {
			return "."
		}
		prefix := sourceName + string(os.PathSeparator)
		if strings.HasPrefix(relative, prefix) {
			return strings.TrimPrefix(relative, prefix)
		}
	}
	return relative
}

func targetWorkspacePath(stateRoot string, ownerUserID string, workspacePath string) (string, bool) {
	ownerUserID = normalizeWorkspaceOwner(ownerUserID)
	legacyRoot := filepath.Clean(filepath.Join(stateRoot, "workspace"))
	normalizedPath := normalizeMigrationPath(workspacePath, stateRoot)
	if normalizedPath == "" {
		return "", false
	}
	targetRoot := appfs.UserWorkspaceRootAt(stateRoot, ownerUserID)
	targetPrefix := filepath.Clean(targetRoot) + string(os.PathSeparator)
	if normalizedPath == filepath.Clean(targetRoot) || strings.HasPrefix(normalizedPath, targetPrefix) {
		return normalizedPath, true
	}
	legacyPrefix := legacyRoot + string(os.PathSeparator)
	if normalizedPath != legacyRoot && !strings.HasPrefix(normalizedPath, legacyPrefix) {
		return "", false
	}

	relative := "."
	if normalizedPath != legacyRoot {
		relative, _ = filepath.Rel(legacyRoot, normalizedPath)
	}
	if ownerUserID != authctx.SystemUserID {
		relative = stripLegacyOwnerPrefix(relative, stateRoot, ownerUserID)
	}
	if relative == "." || relative == "" {
		return targetRoot, true
	}
	return filepath.Join(targetRoot, relative), true
}

func normalizeMigrationPath(path string, stateRoot string) string {
	value := strings.TrimSpace(path)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			value = filepath.Join(home, value[2:])
		}
	}
	if !filepath.IsAbs(value) && strings.TrimSpace(stateRoot) != "" {
		value = filepath.Join(stateRoot, value)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return filepath.Clean(value)
	}
	return filepath.Clean(absolute)
}

// normalizeWorkspaceOwner 去除旧数据中误落库的 JSON 字符串外壳。
//
// 历史 Agent 路径会在拼目录时丢掉双引号，因此 `user_x` 与
// `"user_x"` 会指向同一旧目录。只解码完整且合法的 JSON 字符串，
// 其他 owner 仍保持原值并继续接受路径碰撞检查。
func normalizeWorkspaceOwner(ownerUserID string) string {
	value := strings.TrimSpace(ownerUserID)
	if value == "" {
		return authctx.SystemUserID
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		var decoded string
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			if decoded = strings.TrimSpace(decoded); decoded != "" {
				return decoded
			}
		}
	}
	return value
}

func updateWorkspaceLayoutAgentPaths(
	ctx context.Context,
	db *sql.DB,
	driver string,
	updates []workspaceLayoutAgent,
) error {
	if len(updates) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启 workspace 路径迁移事务: %w", err)
	}
	defer tx.Rollback()

	dialect := storage.NewSQLDialect(driver)
	query := "UPDATE agents SET workspace_path = " + dialect.Bind(1) +
		", owner_user_id = " + dialect.Bind(2) +
		" WHERE id = " + dialect.Bind(3)
	for _, update := range updates {
		if _, err = tx.ExecContext(
			ctx,
			query,
			update.workspacePath,
			update.ownerUserID,
			update.id,
		); err != nil {
			return fmt.Errorf("更新 Agent %s owner 与 workspace 路径: %w", update.id, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交 workspace 路径迁移事务: %w", err)
	}
	return nil
}
