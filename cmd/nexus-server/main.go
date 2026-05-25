package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/infra/syslimit"
	"github.com/nexus-research-lab/nexus/internal/storage"

	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

func runMigrations(cfg config.Config, logger *slog.Logger) error {
	dir := "db/migrations/" + storage.MigrationDirName(cfg.DatabaseDriver)

	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("open db for migration: %w", err)
	}
	defer db.Close()

	if err = goose.SetDialect(storage.GooseDialect(cfg.DatabaseDriver)); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	version, err := goose.GetDBVersion(db)
	if err != nil {
		logger.Info("无法获取当前 migration 版本，尝试初始化", "err", err)
	}

	logger.Info("执行数据库迁移", "current_version", version, "dir", dir)
	version, err = reconcileLegacySQLiteMigrationVersion(db, cfg.DatabaseDriver, version, logger)
	if err != nil {
		return err
	}
	if version > 0 {
		logger.Info("数据库迁移版本就绪", "current_version", version)
	}
	if err = goose.Up(db, dir); err != nil {
		return fmt.Errorf("run goose up: %w", err)
	}
	return nil
}

func buildRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "nexus-server",
		Short:         "启动 Nexus HTTP 服务",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer()
		},
	}
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
