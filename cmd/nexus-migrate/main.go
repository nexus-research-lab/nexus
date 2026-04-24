package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

func main() {
	root := buildRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func buildRootCommand() *cobra.Command {
	cfg := config.Load()

	root := &cobra.Command{
		Use:   "nexus-migrate",
		Short: "Nexus Goose schema migration 工具",
	}

	root.AddCommand(buildGooseCommand(cfg, "up"))
	root.AddCommand(buildGooseCommand(cfg, "down"))
	root.AddCommand(buildGooseCommand(cfg, "status"))
	root.AddCommand(buildGooseCommand(cfg, "version"))
	root.AddCommand(buildCreateCommand(cfg))

	return root
}

func buildGooseCommand(cfg config.Config, action string) *cobra.Command {
	return &cobra.Command{
		Use:   action,
		Short: fmt.Sprintf("执行 %s", action),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, dir, err := openMigrationDB(cfg)
			if err != nil {
				return err
			}
			defer db.Close()

			switch action {
			case "up":
				return goose.UpContext(cmd.Context(), db, dir)
			case "down":
				return goose.DownContext(cmd.Context(), db, dir)
			case "status":
				return goose.StatusContext(cmd.Context(), db, dir)
			case "version":
				version, err := goose.GetDBVersionContext(cmd.Context(), db)
				if err != nil {
					return err
				}
				fmt.Println(version)
				return nil
			default:
				return fmt.Errorf("unsupported action: %s", action)
			}
		},
	}
}

func buildCreateCommand(cfg config.Config) *cobra.Command {
	var name string

	command := &cobra.Command{
		Use:   "create",
		Short: "同时为 sqlite/postgres 创建同版本 SQL migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("migration name is required")
			}

			version := time.Now().UTC().Format("20060102150405")
			for _, driver := range []string{"sqlite", "postgres"} {
				filePath := filepath.Join("db", "migrations", driver, version+"_"+name+".sql")
				content := fmt.Sprintf("-- +goose Up\n-- =====================================================\n-- @File   ：%s\n-- @Date   ：2026/04/10 21:22:41\n-- @Author ：leemysw\n-- 2026/04/10 21:22:41   Create\n-- =====================================================\n\n-- TODO: implement %s migration for %s.\n\n-- +goose Down\n-- TODO: rollback %s migration for %s.\n", filepath.Base(filePath), name, driver, name, driver)
				if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
					return err
				}
				fmt.Println(filePath)
			}
			return nil
		},
	}
	command.Flags().StringVar(&name, "name", "", "migration name")
	return command
}

func openMigrationDB(cfg config.Config) (*sql.DB, string, error) {
	driver := storage.NormalizeSQLDriver(cfg.DatabaseDriver)
	dsn := storage.NormalizeDatabaseURL(cfg.DatabaseURL)
	if err := goose.SetDialect(storage.GooseDialect(cfg.DatabaseDriver)); err != nil {
		return nil, "", err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, "", err
	}

	dir := resolveMigrationDir(cfg.DatabaseDriver)
	return db, dir, nil
}

func resolveMigrationDir(driver string) string {
	return filepath.Join("db", "migrations", storage.MigrationDirName(driver))
}
