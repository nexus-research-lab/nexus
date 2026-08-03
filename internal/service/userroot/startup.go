// INPUT: 启动时已加载的目标配置、已完成 schema migration 的数据库与待切换 users 设置。
// OUTPUT: 离线复制完整 users 数据、切换 Agent 路径；失败时恢复旧根并允许服务继续启动。
// POS: users 根变更的唯一执行阶段，先于任何用户文件迁移和后台服务。
package userroot

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// ReconcileOnStartup 在服务尚未接收请求时提交待切换 users 根。
func ReconcileOnStartup(ctx context.Context, cfg config.Config, logger *slog.Logger) (config.Config, error) {
	if logger == nil {
		logger = slog.Default()
	}
	settings, err := config.LoadRuntimeSettings()
	if err != nil {
		activeRoot := agentsvc.WorkspaceBasePath(cfg)
		if configureErr := appfs.ConfigureUsersRoot(activeRoot); configureErr != nil {
			return cfg, fmt.Errorf("激活 users 根: %w", configureErr)
		}
		logger.Warn("读取 users 根切换状态失败，沿用当前配置", "err", err)
		return cfg, nil
	}
	targetRoot := desiredUsersRoot(cfg, settings)
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return cfg, fmt.Errorf("打开 users 根切换数据库: %w", err)
	}
	defer db.Close()
	records, err := readAgentWorkspaceRecords(ctx, db)
	if err != nil {
		return cfg, err
	}

	sourceRoot, pending, err := resolveAppliedRoot(settings, records, targetRoot)
	if err != nil {
		return cfg, err
	}
	cfg.WorkspacePath = targetRoot
	if targetErr := validateUsersRootTarget(targetRoot); targetErr != nil {
		return recoverDefaultUsersRoot(
			ctx,
			cfg,
			db,
			records,
			settings,
			sourceRoot,
			targetRoot,
			pending,
			logger,
			targetErr,
			"自定义 users 根与宿主 app 数据重叠，已恢复默认根",
		)
	}
	if modeErr := validateUsersRootMode(cfg, targetRoot); modeErr != nil {
		return recoverDefaultUsersRoot(
			ctx,
			cfg,
			db,
			records,
			settings,
			sourceRoot,
			targetRoot,
			pending,
			logger,
			modeErr,
			"自定义 users 根与 runtime isolation enforce 不兼容，已迁回默认根",
		)
	}
	if !pending {
		if err = appfs.ConfigureUsersRoot(targetRoot); err != nil {
			return cfg, fmt.Errorf("激活 users 根: %w", err)
		}
		persistAppliedUsersRoot(settings, targetRoot, logger)
		return cfg, nil
	}
	settings, err = claimMigrationTarget(settings, sourceRoot, targetRoot)
	if err != nil {
		return restoreAppliedRoot(cfg, settings, sourceRoot, logger, err), nil
	}
	if err = appfs.ConfigureUsersRoot(targetRoot); err != nil {
		return restoreAppliedRoot(cfg, settings, sourceRoot, logger, fmt.Errorf("激活 users 根: %w", err)), nil
	}
	if err = applyTransition(ctx, db, records, sourceRoot, targetRoot, cfg.DatabaseDriver); err != nil {
		return restoreAppliedRoot(cfg, settings, sourceRoot, logger, err), nil
	}

	settings.AppliedUsersPath = targetRoot
	settings.WorkspacePath = workspaceSettingForRoot(targetRoot)
	settings.PendingUsersPath = ""
	settings.MigratingUsersPath = ""
	if _, saveErr := config.SaveRuntimeSettings(settings); saveErr != nil {
		// 数据与数据库已经提交到目标根；账本写入失败不应把可启动状态回滚到旧路径。
		logger.Warn("users 根已切换，但更新迁移账本失败", "err", saveErr)
	}
	logger.Info(
		"users 根切换完成",
		"source_root", sourceRoot,
		"target_root", targetRoot,
		"agent_count", len(records),
		"source_preserved", true,
	)
	return cfg, nil
}

func desiredUsersRoot(cfg config.Config, settings config.RuntimeSettings) string {
	if migratingRoot := strings.TrimSpace(settings.MigratingUsersPath); migratingRoot != "" {
		return filepath.Clean(migratingRoot)
	}
	if pendingRoot := strings.TrimSpace(settings.PendingUsersPath); pendingRoot != "" {
		return filepath.Clean(pendingRoot)
	}
	if configuredRoot := strings.TrimSpace(settings.WorkspacePath); configuredRoot != "" {
		return workspaceRootForSetting(cfg, configuredRoot)
	}
	if appliedRoot := strings.TrimSpace(settings.AppliedUsersPath); appliedRoot != "" {
		return filepath.Clean(appliedRoot)
	}
	return agentsvc.WorkspaceBasePath(cfg)
}

func claimMigrationTarget(
	settings config.RuntimeSettings,
	sourceRoot string,
	targetRoot string,
) (config.RuntimeSettings, error) {
	if err := validateWorkspaceRoots(sourceRoot, targetRoot); err != nil {
		return settings, err
	}
	migrationStarted := samePath(settings.MigratingUsersPath, targetRoot)
	allowExisting := migrationStarted || samePath(targetRoot, appfs.DefaultUsersRoot())
	if err := requireMergeableTarget(targetRoot, allowExisting); err != nil {
		return settings, err
	}
	if migrationStarted {
		return settings, nil
	}

	// 先持久化“正在迁移”再接触目标目录。这样启动中断后可以续拷；
	// 尚未被 Nexus 认领的非空目录则始终拒绝覆盖。
	settings.WorkspacePath = workspaceSettingForRoot(sourceRoot)
	settings.AppliedUsersPath = sourceRoot
	settings.PendingUsersPath = ""
	settings.MigratingUsersPath = targetRoot
	saved, err := config.SaveRuntimeSettings(settings)
	if err != nil {
		return settings, fmt.Errorf("记录 users 根迁移状态: %w", err)
	}
	return saved, nil
}

func resolveAppliedRoot(
	settings config.RuntimeSettings,
	records []agentWorkspaceRecord,
	targetRoot string,
) (string, bool, error) {
	appliedRoot := strings.TrimSpace(settings.AppliedUsersPath)
	if appliedRoot != "" {
		if (len(records) == 0 && samePath(appliedRoot, targetRoot)) ||
			agentWorkspaceRecordsMatchRoot(records, targetRoot) {
			return targetRoot, false, nil
		}
		if !samePath(appliedRoot, targetRoot) {
			return appliedRoot, true, nil
		}
	}
	var inferredRoot string
	for _, record := range records {
		if samePath(record.workspacePath, resolveAgentPathAt(targetRoot, record)) {
			continue
		}
		recordRoot, err := inferRecordWorkspaceRoot(record)
		if err != nil {
			return "", false, err
		}
		if inferredRoot == "" {
			inferredRoot = recordRoot
			continue
		}
		if !samePath(inferredRoot, recordRoot) {
			return "", false, fmt.Errorf("既有 Agent workspace 分布在多个历史根目录")
		}
	}
	if inferredRoot != "" {
		return inferredRoot, !samePath(inferredRoot, targetRoot), nil
	}
	if len(records) > 0 {
		return targetRoot, false, nil
	}
	if strings.TrimSpace(settings.WorkspacePath) != "" {
		legacyRoot := appfs.DefaultUsersRoot()
		return legacyRoot, !samePath(legacyRoot, targetRoot), nil
	}
	return targetRoot, false, nil
}

func agentWorkspaceRecordsMatchRoot(records []agentWorkspaceRecord, root string) bool {
	if len(records) == 0 {
		return false
	}
	for _, record := range records {
		if !samePath(record.workspacePath, resolveAgentPathAt(root, record)) {
			return false
		}
	}
	return true
}

func persistAppliedUsersRoot(
	settings config.RuntimeSettings,
	targetRoot string,
	logger *slog.Logger,
) {
	if samePath(settings.AppliedUsersPath, targetRoot) &&
		strings.TrimSpace(settings.PendingUsersPath) == "" &&
		strings.TrimSpace(settings.MigratingUsersPath) == "" &&
		samePath(workspaceRootForSetting(config.Config{}, settings.WorkspacePath), targetRoot) {
		return
	}
	settings.AppliedUsersPath = targetRoot
	settings.WorkspacePath = workspaceSettingForRoot(targetRoot)
	settings.PendingUsersPath = ""
	settings.MigratingUsersPath = ""
	if _, err := config.SaveRuntimeSettings(settings); err != nil {
		logger.Warn("users 根已生效，但补写迁移账本失败", "err", err)
	}
}

func recoverDefaultUsersRoot(
	ctx context.Context,
	cfg config.Config,
	db *sql.DB,
	records []agentWorkspaceRecord,
	settings config.RuntimeSettings,
	sourceRoot string,
	targetRoot string,
	pending bool,
	logger *slog.Logger,
	reason error,
	message string,
) (config.Config, error) {
	fallbackRoot := appfs.DefaultUsersRoot()
	if pending && samePath(sourceRoot, fallbackRoot) {
		settings.WorkspacePath = ""
		settings.AppliedUsersPath = fallbackRoot
		settings.PendingUsersPath = ""
		settings.MigratingUsersPath = ""
		if _, err := config.SaveRuntimeSettings(settings); err != nil {
			return cfg, fmt.Errorf("保存默认 users 根恢复状态: %w", err)
		}
		cfg.WorkspacePath = fallbackRoot
		if err := appfs.ConfigureUsersRoot(fallbackRoot); err != nil {
			return cfg, fmt.Errorf("激活默认 users 根: %w", err)
		}
		logger.Warn(
			message,
			"reason", reason,
			"source_root", sourceRoot,
			"users_root", fallbackRoot,
		)
		return cfg, nil
	}
	migrationSource := targetRoot
	if pending {
		migrationSource = sourceRoot
	}
	if err := applyTransition(
		ctx,
		db,
		records,
		migrationSource,
		fallbackRoot,
		cfg.DatabaseDriver,
	); err != nil {
		return cfg, fmt.Errorf("恢复默认 users 根: %w", err)
	}
	settings.WorkspacePath = ""
	settings.AppliedUsersPath = fallbackRoot
	settings.PendingUsersPath = ""
	settings.MigratingUsersPath = ""
	if _, err := config.SaveRuntimeSettings(settings); err != nil {
		return cfg, fmt.Errorf("保存默认 users 根恢复状态: %w", err)
	}
	cfg.WorkspacePath = fallbackRoot
	if err := appfs.ConfigureUsersRoot(fallbackRoot); err != nil {
		return cfg, fmt.Errorf("激活默认 users 根: %w", err)
	}
	logger.Warn(
		message,
		"reason", reason,
		"source_root", migrationSource,
		"users_root", fallbackRoot,
	)
	return cfg, nil
}

func inferRecordWorkspaceRoot(record agentWorkspaceRecord) (string, error) {
	workspacePath := filepath.Clean(record.workspacePath)
	workspaceDirName := filepath.Base(workspacePath)
	if workspaceDirName == "." || workspaceDirName == string(filepath.Separator) {
		return "", fmt.Errorf("Agent %s 的历史 workspace 目录无效", record.agentID)
	}
	workspaceDirectory := filepath.Dir(workspacePath)
	if !samePath(filepath.Base(workspaceDirectory), "workspace") {
		return "", fmt.Errorf("Agent %s 的历史 workspace 层级无效", record.agentID)
	}
	ownerDirectory := filepath.Dir(workspaceDirectory)
	if !samePath(filepath.Base(ownerDirectory), appfs.UserPathSegment(record.ownerUserID)) {
		return "", fmt.Errorf("Agent %s 的历史 workspace owner 层级无效", record.agentID)
	}
	root := filepath.Dir(ownerDirectory)
	if !samePath(resolveAgentPathAt(root, record), workspacePath) {
		return "", fmt.Errorf("Agent %s 的历史 workspace 路径无法归一化", record.agentID)
	}
	return root, nil
}

func applyTransition(
	ctx context.Context,
	db *sql.DB,
	records []agentWorkspaceRecord,
	sourceRoot string,
	targetRoot string,
	databaseDriver string,
) error {
	if err := validateWorkspaceRoots(sourceRoot, targetRoot); err != nil {
		return err
	}
	if err := validateAgentWorkspaceRecords(records, sourceRoot, targetRoot); err != nil {
		return err
	}
	if err := copyUsersTree(ctx, sourceRoot, targetRoot); err != nil {
		return fmt.Errorf("迁移 users 数据: %w", err)
	}
	if err := ensureTargetAgentDirectories(targetRoot, records); err != nil {
		return fmt.Errorf("准备目标 Agent workspace: %w", err)
	}
	if err := rebaseTranscriptProjects(ctx, sourceRoot, targetRoot, records); err != nil {
		return fmt.Errorf("迁移 runtime transcript 索引: %w", err)
	}
	if err := rewriteRoomWorkspacePaths(ctx, sourceRoot, targetRoot, records); err != nil {
		return fmt.Errorf("重映射 Room workspace 元数据: %w", err)
	}
	return updateAgentWorkspacePaths(ctx, db, records, targetRoot, databaseDriver)
}

func updateAgentWorkspacePaths(
	ctx context.Context,
	db *sql.DB,
	records []agentWorkspaceRecord,
	targetRoot string,
	databaseDriver string,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	dialect := storage.NewSQLDialect(databaseDriver)
	query := fmt.Sprintf(`
UPDATE agents
SET workspace_path = %s, updated_at = %s
WHERE id = %s
  AND COALESCE(NULLIF(TRIM(owner_user_id), ''), '__system__') = %s`,
		dialect.Bind(1),
		dialect.CurrentTimestamp(),
		dialect.Bind(2),
		dialect.Bind(3),
	)
	for _, record := range records {
		targetPath := resolveAgentPathAt(targetRoot, record)
		if samePath(record.workspacePath, targetPath) {
			continue
		}
		result, updateErr := tx.ExecContext(ctx, query,
			targetPath,
			record.agentID,
			record.ownerUserID,
		)
		if updateErr != nil {
			return updateErr
		}
		updated, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if updated != 1 {
			return fmt.Errorf("Agent %s 的 workspace 路径未能原子切换", record.agentID)
		}
	}
	return tx.Commit()
}

func restoreAppliedRoot(
	cfg config.Config,
	settings config.RuntimeSettings,
	sourceRoot string,
	logger *slog.Logger,
	migrationErr error,
) config.Config {
	// 保留用户选择的目标，磁盘空间、权限等瞬时故障可在下次启动继续迁移；
	// 当前进程只回到已生效根，不能把失败目标误记为已生效配置。
	pendingRoot := agentsvc.WorkspaceBasePath(cfg)
	if err := validateWorkspaceRoots(sourceRoot, pendingRoot); err != nil {
		settings.WorkspacePath = workspaceSettingForRoot(sourceRoot)
		settings.PendingUsersPath = ""
		settings.MigratingUsersPath = ""
	}
	settings.AppliedUsersPath = sourceRoot
	if _, err := config.SaveRuntimeSettings(settings); err != nil {
		logger.Error(
			"users 根切换失败且恢复设置未写入；本次仍使用原根启动",
			"migration_error", migrationErr,
			"restore_error", err,
		)
	} else {
		logger.Warn(
			"users 根迁移失败，已沿用原根启动并保留待迁移目标",
			"err", migrationErr,
			"restored_root", sourceRoot,
			"pending_root", pendingRoot,
		)
	}
	cfg.WorkspacePath = sourceRoot
	if err := appfs.ConfigureUsersRoot(sourceRoot); err != nil {
		logger.Error("恢复 users 根进程路径失败", "err", err, "users_root", sourceRoot)
	}
	return cfg
}
