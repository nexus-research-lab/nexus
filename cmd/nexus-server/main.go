// INPUT: Nexus server 环境配置、桌面状态根独占锁、数据库 migration、进程生命周期信号或显式 runtime 检查参数。
// OUTPUT: 完成 schema/宿主修复的 HTTP/WebSocket 服务，或不启动服务的随包 runtime 兼容性报告。
// POS: nexus-server 可执行入口，只装配启动阶段，不承载领域规则。
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app/runtimecheck"
	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/appdelta"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/desktopinstance"
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
	root.AddCommand(buildRuntimeCheckCommand(), &cobra.Command{Use: "apply-update-delta PATCH FROM TO INSTALLED STAGE", Hidden: true, Args: cobra.ExactArgs(5), RunE: func(_ *cobra.Command, a []string) error { return appdelta.ApplyFile(a[0], a[1], a[2], a[3], a[4]) }})
	return root
}

func buildRuntimeCheckCommand() *cobra.Command {
	var binary string
	command := &cobra.Command{
		Use: "check-desktop-runtime", Short: "检查随包 nxs 与当前 App 的能力兼容性（不调用模型）",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			report, err := runtimecheck.Check(ctx, binary)
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err)
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
		},
	}
	command.Flags().StringVar(&binary, "nxs", "", "安装包内 nxs 的绝对路径")
	_ = command.MarkFlagRequired("nxs")
	return command
}

func runServer() error {
	// 先加载宿主环境，再读取 canonical 数据库与日志路径。
	if err := config.LoadDotEnv(); err != nil {
		return fmt.Errorf("加载环境配置失败: %w", err)
	}
	stateRoot := appfs.StateRoot()
	// 窗口进程的锁不能证明旧 sidecar 已退出。先由本进程持有 app 根锁，
	// 再执行布局/数据库迁移；defer 晚于下面的服务 Close 执行。
	if strings.EqualFold(strings.TrimSpace(os.Getenv("NEXUS_APP_MODE")), "desktop") {
		instance, err := desktopinstance.AcquireForPlatform(stateRoot)
		if err != nil {
			return fmt.Errorf("acquire desktop sidecar ownership: %w", err)
		}
		if instance != nil {
			defer instance.Close()
		}
	}
	cfg := config.Load()
	hostKeys, err := credentials.ResolveHostKeys(
		cfg.ConnectorCredentialsHostKeyMode,
		stateRoot,
		cfg.ConnectorCredentialsKey,
		cfg.ConnectorCredentialsLegacyKeys,
	)
	if err != nil {
		return fmt.Errorf("解析 Connector credentials host keys: %w", err)
	}
	cfg.ConnectorCredentialsKey = hostKeys.Active
	cfg.ConnectorCredentialsLegacyKeys = hostKeys.Legacy
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
	if err := authsvc.ValidateControlConfig(cfg); err != nil {
		logger.Error("Control 配置无效", "err", err)
		return err
	}
	logger.Info("Connector credentials host keys 就绪",
		"source", hostKeys.Source,
		"legacy_keys", len(hostKeys.Legacy),
	)

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
	// 桌面宿主先离线复制整个状态根；新实例在任何业务服务启动前提交绝对路径重映射。
	// 这里失败会让宿主保留旧根并自动回退，不能带着一半迁移的数据继续启动。
	if err := migration.RunDesktopStateRootRebase(context.Background(), cfg, logger); err != nil {
		logger.Error("桌面状态根迁移提交失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := migration.RunWorkspaceFiles(appfs.AppDir(), agentsvc.WorkspaceBasePath(cfg), logger); err != nil {
		logger.Error("工作区文件迁移失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	if runtime.GOOS == "windows" && cfg.AppMode == "desktop" && os.Getenv("NEXUS_DESKTOP_PARENT_PIPE") == "1" {
		removed, err := desktopinstance.CleanupWindowsScratch(stateRoot)
		if err != nil {
			logger.Warn("Windows scratch cleanup incomplete; retained entries will be retried on next startup", "removed", removed, "err", err)
		}
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
	if _, err := migration.RunConnectorCredentialKeyMigration(context.Background(), cfg, logger); err != nil {
		logger.Error("Connector credentials key migration 失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
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
	if runtime.GOOS == "windows" && os.Getenv("NEXUS_DESKTOP_PARENT_PIPE") == "1" {
		desktopinstance.WatchParentInput(os.Stdin, stop)
	}
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
