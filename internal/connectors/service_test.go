// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service_test.go
// @Date   ：2026/04/11 15:52:00
// @Author ：leemysw
// 2026/04/11 15:52:00   Create
// =====================================================

package connectors

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestServiceListsConnectorsAndBuildsAuthURL(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	service := NewService(cfg, db)
	ctx := context.Background()

	items, err := service.ListConnectors(ctx, "github", "", "")
	if err != nil {
		t.Fatalf("列出连接器失败: %v", err)
	}
	if len(items) != 1 || items[0].ConnectorID != "github" {
		t.Fatalf("连接器过滤结果不正确: %+v", items)
	}
	if !items[0].IsConfigured {
		t.Fatalf("GitHub 连接器应视为已配置: %+v", items[0])
	}

	authURL, err := service.GetAuthURL(ctx, "github", "")
	if err != nil {
		t.Fatalf("生成授权地址失败: %v", err)
	}
	parsedURL, err := url.Parse(authURL.AuthURL)
	if err != nil {
		t.Fatalf("解析授权地址失败: %v", err)
	}
	if parsedURL.Query().Get("client_id") != cfg.ConnectorGitHubClientID {
		t.Fatalf("client_id 未写入授权地址: %s", authURL.AuthURL)
	}
	if strings.TrimSpace(authURL.State) == "" {
		t.Fatalf("state 不能为空: %+v", authURL)
	}

	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"token"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}
	count, err := service.GetConnectedCount(ctx)
	if err != nil {
		t.Fatalf("读取已连接数量失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("已连接数量不正确: got=%d want=1", count)
	}
}

func newConnectorsTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		Host:                        "127.0.0.1",
		Port:                        18013,
		ProjectName:                 "nexus-connectors-test",
		APIPrefix:                   "/agent/v1",
		WebSocketPath:               "/agent/v1/chat/ws",
		DefaultAgentID:              "nexus",
		WorkspacePath:               filepath.Join(root, "workspace"),
		CacheFileDir:                filepath.Join(root, "cache"),
		DatabaseDriver:              "sqlite",
		DatabaseURL:                 filepath.Join(root, "nexus.db"),
		ConnectorOAuthRedirectURI:   "http://localhost:3000/capability/connectors",
		ConnectorGitHubClientID:     "github-client-id",
		ConnectorGitHubClientSecret: "github-client-secret",
	}
}

func migrateConnectorsSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, connectorsTestMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func connectorsTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
