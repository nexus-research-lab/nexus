// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：auth_commands_test.go
// @Date   ：2026/04/12 10:42:00
// @Author ：leemysw
// 2026/04/12 10:42:00   Create
// =====================================================

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestAuthAndUserCommands(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	statusPayload := runCLICommand(t, cfg, "auth", "status")
	statusItem := asMap(t, statusPayload["item"])
	if !asBool(t, statusItem["setup_required"]) {
		t.Fatalf("初始 auth status 应要求 setup: %+v", statusItem)
	}

	initPayload := runCLICommand(
		t,
		cfg,
		"auth",
		"init-owner",
		"--username",
		"admin",
		"--display-name",
		"系统管理员",
		"--password",
		"password123",
	)
	initItem := asMap(t, initPayload["item"])
	if initItem["username"] != "admin" {
		t.Fatalf("init-owner 返回数据不正确: %+v", initItem)
	}

	listPayload := runCLICommand(t, cfg, "user", "list")
	items, ok := listPayload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("user list 结果不正确: %+v", listPayload)
	}

	resetPayload := runCLICommand(
		t,
		cfg,
		"user",
		"reset-password",
		"--username",
		"admin",
		"--password",
		"password456",
	)
	resetItem := asMap(t, resetPayload["item"])
	if resetItem["username"] != "admin" {
		t.Fatalf("reset-password 返回数据不正确: %+v", resetItem)
	}
}

func runCLICommand(t *testing.T, cfg config.Config, args ...string) map[string]any {
	t.Helper()

	command, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 CLI 命令失败: %v", err)
	}
	command.SetArgs(args)

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建 stdout 管道失败: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	executeErr := command.Execute()
	_ = writer.Close()

	var buffer bytes.Buffer
	if _, err = buffer.ReadFrom(reader); err != nil {
		t.Fatalf("读取 CLI 输出失败: %v", err)
	}
	_ = reader.Close()

	if executeErr != nil {
		t.Fatalf("执行 CLI 命令失败: %v, output=%s", executeErr, buffer.String())
	}

	var payload map[string]any
	if err = json.Unmarshal(buffer.Bytes(), &payload); err != nil {
		t.Fatalf("解析 CLI JSON 输出失败: %v, output=%s", err, buffer.String())
	}
	return payload
}

func newCLITestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18032,
		ProjectName:    "nexus-cli-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateCLISQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开 CLI 测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, cliMigrationDir(t)); err != nil {
		t.Fatalf("执行 CLI migration 失败: %v", err)
	}
}

func cliMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 CLI 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()

	item, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("输出结构不是对象: %#v", value)
	}
	return item
}

func asBool(t *testing.T, value any) bool {
	t.Helper()

	item, ok := value.(bool)
	if !ok {
		t.Fatalf("输出结构不是布尔值: %#v", value)
	}
	return item
}
