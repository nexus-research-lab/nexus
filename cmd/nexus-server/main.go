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

	db, err := storage.OpenDB(cfg)
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

	version, err := goose.GetDBVersion(db)
	if err != nil {
		logger.Info("无法获取当前 migration 版本，尝试初始化", "err", err)
	}

	logger.Info("执行数据库迁移", "current_version", version, "dir", dir)
	if version > 0 {
		logger.Info("数据库迁移版本就绪", "current_version", version)
	}
	if err = goose.Up(db, dir); err != nil {
		return fmt.Errorf("run goose up: %w", err)
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
	normalizedUsername := strings.ToLower(strings.TrimSpace(username))
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
	if err := migration.RunWorkspaceFiles(appfs.ConfigDir(), agentsvc.WorkspaceBasePath(cfg), logger); err != nil {
		logger.Error("工作区文件迁移失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := ensureOwnerFromEnv(context.Background(), cfg, logger); err != nil {
		logger.Error("owner bootstrap 失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	// Provider scope 补偿只属于桌面 App 的本地 SQLite，Web/服务器部署不触碰用户数据。
	if strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") && storage.IsSQLiteSQLDriver(cfg.DatabaseDriver) {
		if err := migration.RepairDesktopProviderScope(context.Background(), cfg, logger); err != nil {
			logger.Error("桌面 Provider scope 补偿迁移失败", "err", err)
			_, _ = fmt.Fprintln(os.Stderr, err)
			return err
		}
	}

	server, err := serverapp.NewWithLogger(cfg, logger)
	if err != nil {
		logger.Error("初始化 HTTP 服务失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
