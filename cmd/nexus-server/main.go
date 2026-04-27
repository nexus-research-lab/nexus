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
	"syscall"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/storage"

	"github.com/pressly/goose/v3"
)

func runMigrations(cfg config.Config, logger *slog.Logger) error {
	driver := storage.NormalizeSQLDriver(cfg.DatabaseDriver)
	dsn := storage.NormalizeDatabaseURL(cfg.DatabaseURL)
	dir := "db/migrations/" + storage.MigrationDirName(cfg.DatabaseDriver)

	db, err := sql.Open(driver, dsn)
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
	if err = goose.Up(db, dir); err != nil {
		return fmt.Errorf("run goose up: %w", err)
	}
	return nil
}

func main() {
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

	// 自动运行 schema migrations，确保首次启动或升级时数据库 schema 就绪。
	if err := runMigrations(cfg, logger); err != nil {
		logger.Error("数据库迁移失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	server, err := serverapp.NewWithLogger(cfg, logger)
	if err != nil {
		logger.Error("初始化 HTTP 服务失败", "err", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
		os.Exit(1)
	}
	logger.Info("服务已停止")
}
