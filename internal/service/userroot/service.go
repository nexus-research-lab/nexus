// INPUT: 当前生效配置、用户请求的新 users 根与 Agent 路径投影。
// OUTPUT: 只登记待重启迁移的目标，不在运行中的服务里复制用户数据。
// POS: users 根变更的在线调度阶段；实际迁移只发生在下次启动的无并发窗口。
package userroot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

// Manager 串行登记 users 根迁移，避免并发保存互相覆盖目标。
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

// Schedule 校验新 users 根并登记迁移目标；当前进程、用户文件与数据库均保持不变。
func (m *Manager) Schedule(ctx context.Context, requestedPath string) (config.RuntimeSettings, error) {
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
	resumeStarted := samePath(existing.MigratingUsersPath, targetRoot) &&
		samePath(existing.AppliedUsersPath, currentRoot)
	allowExistingTarget := samePath(currentRoot, targetRoot) ||
		resumeStarted ||
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
	existing.WorkspacePath = workspaceSettingForRoot(currentRoot)
	existing.AppliedUsersPath = currentRoot
	existing.PendingUsersPath = targetRoot
	existing.MigratingUsersPath = ""
	if resumeStarted {
		existing.PendingUsersPath = ""
		existing.MigratingUsersPath = targetRoot
	}
	if samePath(currentRoot, targetRoot) {
		existing.WorkspacePath = workspaceSettingForRoot(targetRoot)
		existing.PendingUsersPath = ""
		existing.MigratingUsersPath = ""
	}
	settings, err := config.SaveRuntimeSettings(existing)
	if err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("登记 users 根迁移: %w", err)
	}
	m.loggerFor(ctx).Info(
		"users 根迁移已登记，等待重启执行",
		"source_root", currentRoot,
		"target_root", targetRoot,
		"agent_count", len(records),
	)
	return settings, nil
}

func workspaceSettingForRoot(root string) string {
	if samePath(root, appfs.DefaultUsersRoot()) {
		return ""
	}
	return filepath.Clean(strings.TrimSpace(root))
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
