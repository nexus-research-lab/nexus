// INPUT: 当前生效配置、用户请求的新 users 根与 Agent 路径投影。
// OUTPUT: 先完成 owner 数据预迁移，再持久化待重启切换的宿主设置。
// POS: users 根变更的在线阶段；不改当前进程使用的配置或数据库 Agent 路径。
package userroot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

// Manager 串行协调 users 根预迁移，避免并发保存互相覆盖目标。
type Manager struct {
	config config.Config
	db     *sql.DB
	logger *slog.Logger
	mu     sync.Mutex
}

// NewManager 创建 users 根迁移器。
func NewManager(cfg config.Config, db *sql.DB, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	return &Manager{config: cfg, db: db, logger: logger}
}

// Stage 在写入新设置前复制当前 users 根；数据库投影留到无并发的下次启动切换。
func (m *Manager) Stage(ctx context.Context, requestedPath string) (config.RuntimeSettings, error) {
	if m == nil || m.db == nil {
		return config.RuntimeSettings{}, errors.New("users root migration manager is unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	currentRoot := agentsvc.WorkspaceBasePath(m.config)
	targetRoot := workspaceRootForSetting(m.config, requestedPath)
	if err := validateWorkspaceRoots(currentRoot, targetRoot); err != nil {
		return config.RuntimeSettings{}, err
	}
	if err := validateUsersRootMode(m.config, targetRoot); err != nil {
		return config.RuntimeSettings{}, err
	}

	existing, err := config.LoadRuntimeSettings()
	if err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("读取当前 users 根设置: %w", err)
	}
	retry := samePath(existing.StagingUsersPath, targetRoot) ||
		(samePath(workspaceRootForSetting(m.config, existing.WorkspacePath), targetRoot) &&
			samePath(existing.AppliedUsersPath, currentRoot))
	allowExistingTarget := samePath(currentRoot, targetRoot) ||
		retry ||
		samePath(targetRoot, appfs.DefaultUsersRoot())
	if err = requireMergeableTarget(targetRoot, allowExistingTarget); err != nil {
		return config.RuntimeSettings{}, err
	}

	records, err := readAgentWorkspaceRecords(ctx, m.db)
	if err != nil {
		return config.RuntimeSettings{}, err
	}
	if err = validateAgentWorkspaceRecords(records, currentRoot, targetRoot); err != nil {
		return config.RuntimeSettings{}, err
	}
	existing.StagingUsersPath = targetRoot
	if _, err = config.SaveRuntimeSettings(existing); err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("记录 users 根预迁移状态: %w", err)
	}
	if !samePath(currentRoot, targetRoot) {
		if err = os.MkdirAll(currentRoot, 0o700); err != nil {
			return config.RuntimeSettings{}, fmt.Errorf("准备当前 users 根: %w", err)
		}
		if err = copyUsersTree(ctx, currentRoot, targetRoot); err != nil {
			return config.RuntimeSettings{}, fmt.Errorf("迁移现有 users 数据: %w", err)
		}
	}
	if err = ensureTargetAgentDirectories(targetRoot, records); err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("校验新 users 根: %w", err)
	}
	if err = rebaseTranscriptProjects(ctx, currentRoot, targetRoot, records); err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("迁移 runtime transcript 索引: %w", err)
	}
	if err = rewriteRoomWorkspacePaths(ctx, currentRoot, targetRoot, records); err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("重映射 Room workspace 元数据: %w", err)
	}

	desiredPath := targetRoot
	if strings.TrimSpace(requestedPath) == "" {
		desiredPath = ""
	}
	existing.WorkspacePath = desiredPath
	existing.AppliedUsersPath = currentRoot
	existing.StagingUsersPath = ""
	settings, err := config.SaveRuntimeSettings(existing)
	if err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("保存 users 根切换状态: %w", err)
	}
	m.loggerFor(ctx).Info(
		"users 根预迁移完成，等待重启切换",
		"source_root", currentRoot,
		"target_root", targetRoot,
		"agent_count", len(records),
		"source_preserved", true,
	)
	return settings, nil
}

func (m *Manager) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, m.logger)
}

func workspaceRootForSetting(base config.Config, workspacePath string) string {
	if strings.TrimSpace(workspacePath) == "" {
		return appfs.DefaultUsersRoot()
	}
	configured := base
	configured.WorkspacePath = strings.TrimSpace(workspacePath)
	return agentsvc.WorkspaceBasePath(configured)
}

func validateUsersRootMode(cfg config.Config, usersRoot string) error {
	if !strings.EqualFold(strings.TrimSpace(cfg.RuntimeIsolationMode), "enforce") ||
		samePath(usersRoot, appfs.DefaultUsersRoot()) {
		return nil
	}
	return errors.New("runtime isolation enforce 使用 root-owned 默认 users 根，不能从应用内迁移")
}
