package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

func TestBuildRootCommandHelpDoesNotRunServer(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	buf := new(bytes.Buffer)
	cmd := buildRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	os.Args = []string{"nexus-server", "--help"}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help to exit cleanly, got error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected help output, got empty string")
	}
	if bytes.Contains(buf.Bytes(), []byte("run goose up")) {
		t.Fatal("help output unexpectedly contains migration failure")
	}
	if bytes.Contains(buf.Bytes(), []byte("migrate")) {
		t.Fatal("help output should not expose a manual migrate subcommand")
	}
}

func TestEnsureOwnerFromEnvBootstrapsOwnerIdempotently(t *testing.T) {
	cfg := testServerConfig(t)
	logger := discardLogger()
	if err := runMigrations(cfg, logger); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}

	t.Setenv(authInitOwnerUsernameEnvName, "Admin")
	t.Setenv(authInitOwnerDisplayNameEnvName, "Root Admin")
	t.Setenv(authInitOwnerPasswordEnvName, "password123")
	if err := ensureOwnerFromEnv(context.Background(), cfg, logger); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}
	if err := ensureOwnerFromEnv(context.Background(), cfg, logger); err != nil {
		t.Fatalf("重复初始化 owner 应保持幂等: %v", err)
	}

	users := listAuthUsers(t, cfg)
	if len(users) != 1 {
		t.Fatalf("owner 初始化应只创建一个用户: %+v", users)
	}
	if users[0].Username != "admin" || users[0].DisplayName != "Root Admin" || users[0].Role != authsvc.RoleOwner {
		t.Fatalf("owner 用户不符合预期: %+v", users[0])
	}
}

func TestEnsureOwnerFromEnvRequiresPasswordWhenProfileProvided(t *testing.T) {
	cfg := testServerConfig(t)
	logger := discardLogger()
	if err := runMigrations(cfg, logger); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}

	t.Setenv(authInitOwnerUsernameEnvName, "admin")
	t.Setenv(authInitOwnerPasswordEnvName, "")
	err := ensureOwnerFromEnv(context.Background(), cfg, logger)
	if err == nil || !strings.Contains(err.Error(), authInitOwnerPasswordEnvName) {
		t.Fatalf("缺少密码时应返回明确错误: %v", err)
	}
}

func testServerConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "nexus.db"),
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func listAuthUsers(t *testing.T, cfg config.Config) []authsvc.User {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	service := authsvc.NewServiceWithDB(cfg, db)
	users, err := service.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	return users
}
