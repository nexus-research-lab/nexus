// INPUT: Nexus server 环境配置、数据库 migration 与进程生命周期信号。
// OUTPUT: 完成 schema/宿主修复后启动并完整收口的 HTTP/WebSocket 服务。
// POS: nexus-server 可执行入口，只装配启动阶段，不承载领域规则。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/infra/syslimit"
	"github.com/nexus-research-lab/nexus/internal/migration"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage"

	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

const (
	authInitOwnerUsernameEnvName    = "AUTH_INIT_OWNER_USERNAME"
	authInitOwnerDisplayNameEnvName = "AUTH_INIT_OWNER_DISPLAY_NAME"
	authInitOwnerPasswordEnvName    = "AUTH_INIT_OWNER_PASSWORD"
)

func openMigrationDB(cfg config.Config) (*sql.DB, string, error) {
	dir := filepath.Join(appfs.Root(), "db", "migrations", storage.MigrationDirName(cfg.DatabaseDriver))

	db, err := storage.OpenMigrationDB(cfg)
	if err != nil {
		return nil, "", fmt.Errorf("open db for migration: %w", err)
	}

	if err = goose.SetDialect(storage.GooseDialect(cfg.DatabaseDriver)); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("set goose dialect: %w", err)
	}
	return db, dir, nil
}

func runMigrations(cfg config.Config, logger *slog.Logger) error {
	db, dir, err := openMigrationDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	allowMissing := false
	version, err := goose.GetDBVersion(db)
	if err != nil {
		logger.Info("无法获取当前 migration 版本，尝试初始化", "err", err)
	} else {
		if err = migration.RepairLegacyAgentDisabledSkillSchema(
			context.Background(),
			cfg.DatabaseDriver,
			db,
			version,
			logger,
		); err != nil {
			return fmt.Errorf("repair legacy migration version collision: %w", err)
		}
		version, err = goose.GetDBVersion(db)
		if err != nil {
			return fmt.Errorf("read migration version after compatibility repair: %w", err)
		}
		automationPermissionReplay, repairErr := migration.RepairLegacyAutomationPermissionMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, version, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("repair automation permission migration version collision: %w", repairErr)
		}
		allowMissing = allowMissing || automationPermissionReplay
		version, err = goose.GetDBVersion(db)
		if err != nil {
			return fmt.Errorf("read migration version after automation permission repair: %w", err)
		}
		privateSkillReplay, repairErr := migration.RepairLegacyPrivateSkillMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, version, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("repair private Skill migration version collision: %w", repairErr)
		}
		allowMissing = allowMissing || privateSkillReplay
		version, err = goose.GetDBVersion(db)
		if err != nil {
			return fmt.Errorf("read migration version after private Skill repair: %w", err)
		}
		goalMigrationReplay, repairErr := migration.RepairLegacyGoalMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("repair Goal migration version collision: %w", repairErr)
		}
		allowMissing = allowMissing || goalMigrationReplay
		version, err = goose.GetDBVersion(db)
		if err != nil {
			return fmt.Errorf("read migration version after Goal repair: %w", err)
		}
		if !allowMissing {
			if err = migration.RepairLegacyExecutionIdentityClaimSchema(
				context.Background(),
				cfg.DatabaseDriver,
				db,
				version,
				logger,
			); err != nil {
				return fmt.Errorf("repair legacy execution identity claim schema: %w", err)
			}
		}
		agentCreationReplay, repairErr := migration.RepairLegacyAgentCreationMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("repair shifted recovery migration version collision: %w", repairErr)
		}
		allowMissing = allowMissing || agentCreationReplay
	}

	logger.Info("执行数据库迁移", "current_version", version, "dir", dir)
	if version > 0 {
		logger.Info("数据库迁移版本就绪", "current_version", version)
	}
	var options []goose.OptionsFunc
	if allowMissing {
		options = append(options, goose.WithAllowMissing())
	}
	if err = goose.Up(db, dir, options...); err != nil {
		return fmt.Errorf("run goose up: %w", err)
	}
	if allowMissing {
		version, err = goose.GetDBVersion(db)
		if err != nil {
			return fmt.Errorf("read migration version after compatibility replay: %w", err)
		}
		automationPending, repairErr := migration.RepairLegacyAutomationPermissionMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, version, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("finalize automation permission migration collision repair: %w", repairErr)
		}
		if automationPending {
			return errors.New("automation permission migration collision repair remains incomplete")
		}
		version, err = goose.GetDBVersion(db)
		if err != nil {
			return fmt.Errorf("read migration version after automation permission repair finalization: %w", err)
		}
		pending, repairErr := migration.RepairLegacyPrivateSkillMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, version, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("finalize private Skill migration collision repair: %w", repairErr)
		}
		if pending {
			return errors.New("private Skill migration collision repair remains incomplete")
		}
		goalPending, repairErr := migration.RepairLegacyGoalMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("finalize Goal migration collision repair: %w", repairErr)
		}
		if goalPending {
			return errors.New("Goal migration collision repair remains incomplete")
		}
		agentCreationPending, repairErr := migration.RepairLegacyAgentCreationMigrationCollision(
			context.Background(), cfg.DatabaseDriver, db, logger,
		)
		if repairErr != nil {
			return fmt.Errorf("finalize shifted recovery migration collision repair: %w", repairErr)
		}
		if agentCreationPending {
			return errors.New("shifted recovery migration collision repair remains incomplete")
		}
	}
	return nil
}

func ensureOwnerFromEnv(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	password := os.Getenv(authInitOwnerPasswordEnvName)
	username := strings.TrimSpace(os.Getenv(authInitOwnerUsernameEnvName))
	displayName := strings.TrimSpace(os.Getenv(authInitOwnerDisplayNameEnvName))
	if strings.TrimSpace(password) == "" {
		if username != "" || displayName != "" {
			return fmt.Errorf("%s is required when %s or %s is set",
				authInitOwnerPasswordEnvName,
				authInitOwnerUsernameEnvName,
				authInitOwnerDisplayNameEnvName,
			)
		}
		logger.Info("未配置 owner 初始化密码，跳过 owner bootstrap")
		return nil
	}
	if username == "" {
		username = "admin"
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("open db for owner bootstrap: %w", err)
	}
	defer db.Close()

	authService := authsvc.NewServiceWithDB(cfg, db)
	users, err := authService.ListUsers(ctx)
	if err != nil {
		return err
	}

	hasActiveAdmin := false
	targetUserRole := ""
	normalizedUsername := strings.ToLower(username)
	for _, user := range users {
		if user.Username == normalizedUsername {
			targetUserRole = user.Role
		}
		if user.Status == authsvc.UserStatusActive && (user.Role == authsvc.RoleOwner || user.Role == authsvc.RoleAdmin) {
			hasActiveAdmin = true
		}
	}
	if hasActiveAdmin {
		logger.Info("owner/admin 用户已存在，跳过 owner bootstrap")
		return nil
	}

	if len(users) == 0 {
		user, err := authService.InitOwner(ctx, authsvc.InitOwnerInput{
			Username:    username,
			DisplayName: displayName,
			Password:    password,
		})
		if err != nil {
			return err
		}
		logger.Info("已初始化首个 owner 用户", "username", user.Username)
		return nil
	}

	if targetUserRole != "" {
		return fmt.Errorf("bootstrap username %s already exists with role %s, but no active owner/admin account was found",
			username,
			targetUserRole,
		)
	}
	user, err := authService.CreateUser(ctx, authsvc.CreateUserInput{
		Username:    username,
		DisplayName: displayName,
		Password:    password,
		Role:        authsvc.RoleOwner,
	})
	if err != nil {
		return err
	}
	logger.Info("已有用户但无 active owner/admin，已创建 owner 用户", "username", user.Username)
	return nil
}

func buildRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "nexus-server",
		Short:         "启动 Nexus HTTP 服务",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer()
		},
	}
	return root
}

func runServer() error {
	// 先加载宿主环境并恢复显式版本化的旧布局，再读取 canonical 数据库与日志路径。
	// 这一步必须早于 config.Load；否则 v0.1.30 迁移缺口会在 app/data 误建空数据库。
	if err := config.LoadDotEnv(); err != nil {
		return fmt.Errorf("加载环境配置失败: %w", err)
	}
	stateRoot := appfs.StateRoot()
	if err := migration.RunStateLayout(stateRoot, slog.Default()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}

	cfg := config.Load()
	logger := logx.New(logx.Options{
		Service: cfg.ProjectName,
		Level:   cfg.LogLevel,
		Format:  cfg.LogFormat,
		Stdout:  cfg.LogStdout,
		NoColor: cfg.LogNoColor,
		File: logx.FileOptions{
			Enabled:     cfg.LogFileEnabled,
			Path:        cfg.LogPath,
			RotateDaily: cfg.LogRotateDaily,
			MaxSizeMB:   cfg.LogMaxSizeMB,
			MaxAgeDays:  cfg.LogMaxAgeDays,
			MaxBackups:  cfg.LogMaxBackups,
			Compress:    cfg.LogCompress,
		},
	})

	limitSnapshot, limitErr := syslimit.EnsureOpenFilesLimit(8192)
	if limitErr != nil {
		logger.Warn("提升文件句柄限制失败", "err", limitErr)
	} else if limitSnapshot.Soft > 0 {
		logger.Info("文件句柄限制就绪",
			"soft_limit", limitSnapshot.Soft,
			"hard_limit", limitSnapshot.Hard,
			"raised", limitSnapshot.Raised,
		)
	}

	// 自动运行 schema migrations，确保首次启动或升级时数据库 schema 就绪。
	if err := runMigrations(cfg, logger); err != nil {
		logger.Error("数据库迁移失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	// 受影响版本可能已在 canonical 路径积累少量新记录。旧库仍是历史真相，
	// 新库只补入不冲突记录；合并失败时隔离副本保持完整，不能阻断旧数据恢复。
	if err := migration.MergeSkippedStateLayoutDatabase(context.Background(), cfg, logger); err != nil {
		logger.Warn("状态布局迁移缺口的隔离数据库未自动合并，已保留完整副本", "err", err)
	}
	// 桌面宿主先离线复制整个状态根；新实例在任何业务服务启动前提交绝对路径重映射。
	// 这里失败会让宿主保留旧根并自动回退，不能带着一半迁移的数据继续启动。
	if err := migration.RunDesktopStateRootRebase(context.Background(), cfg, logger); err != nil {
		logger.Error("桌面状态根迁移提交失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := migration.RunWorkspaceLayout(context.Background(), cfg, stateRoot, logger); err != nil {
		logger.Error("workspace 布局迁移失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := migration.MergeSkippedStateLayoutUsers(stateRoot, logger); err != nil {
		logger.Warn("状态布局迁移缺口的用户文件未自动合并，已保留完整隔离数据", "err", err)
	}
	if err := migration.RunWorkspaceFiles(appfs.AppDir(), agentsvc.WorkspaceBasePath(cfg), logger); err != nil {
		logger.Error("工作区文件迁移失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := ensureOwnerFromEnv(context.Background(), cfg, logger); err != nil {
		logger.Error("owner bootstrap 失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	// 平台 Skill 是 enforce runtime 的显式只读根，必须先完成原子发布，
	// 再由 launcher 收紧 ACL；不能把首次发布推迟到并发的聊天请求中。
	if err := workspacepkg.EnsurePlatformSkillLibrary(); err != nil {
		logger.Error("平台 Skill 库初始化失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := workspacepkg.EnsureRuntimeCLICommands(); err != nil {
		logger.Error("Agent runtime CLI 初始化失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	// 宿主 Skill 是桌面可选来源。只在启动窗口创建稳定根，
	// 内容校验与刷新交给后台 watcher，不应阻断服务健康。
	if err := workspacepkg.PrepareHostSkillLibrary(cfg); err != nil {
		logger.Warn("宿主 Skill 兼容根准备失败，继续启动服务", "err", err)
	}
	if err := migration.RunRuntimeIdentitySync(context.Background(), cfg, logger); err != nil {
		logger.Error("runtime OS identity 同步失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := migration.RunRoomFiles(context.Background(), cfg, logger); err != nil {
		// enforce 部署已先收紧 owner state 权限；迁移失败时保留尚未处理的旧文件，
		// 单条历史脏数据不能阻断服务。
		logger.Warn("Room 文件状态迁移未完成，将在下次启动重试", "err", err)
	}
	// Provider scope 补偿只属于桌面 App 的本地 SQLite，Web/服务器部署不触碰用户数据。
	if strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") && storage.IsSQLiteSQLDriver(cfg.DatabaseDriver) {
		if err := migration.RepairDesktopProviderScope(context.Background(), cfg, logger); err != nil {
			logger.Error("桌面 Provider scope 补偿迁移失败", "err", err)
			_, _ = fmt.Fprintln(os.Stderr, err)
			return err
		}
	}
	// 旧版本允许积累多个未产生用户输入的 Session。只在桌面 SQLite 的无并发启动窗口
	// 自动收口一次；修复失败保留 started 标记并继续启动，后续由显式维护命令处理。
	if err := migration.RunDesktopLegacyConversationDraftRepair(context.Background(), cfg, logger); err != nil {
		logger.Warn("历史空白 Session 一次性兼容修复未完成，继续启动服务", "err", err)
	}

	server, err := serverapp.NewWithLogger(cfg, logger)
	if err != nil {
		logger.Error("初始化 HTTP 服务失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	defer func() {
		if closeErr := server.Close(context.Background()); closeErr != nil {
			logger.Warn("服务资源关闭失败", "err", closeErr)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go workspacepkg.WatchHostSkillLibrary(
		ctx,
		cfg,
		logger.With("component", "workspace.host_skills"),
	)

	logger.Info("服务启动中",
		"addr", cfg.Address(),
		"database_driver", cfg.DatabaseDriver,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
	)
	if err = server.ListenAndServe(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("服务异常退出", "err", err)
		return err
	}
	logger.Info("服务已停止")
	return nil
}

func main() {
	root := buildRootCommand()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
