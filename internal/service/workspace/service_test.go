package workspace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestServiceManagesWorkspaceFiles(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "工作区测试助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	files, err := workspaceService.ListFiles(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("列出 workspace 文件失败: %v", err)
	}
	if !containsWorkspacePath(files, "AGENTS.md") {
		t.Fatalf("初始化模板未生成 AGENTS.md: %+v", files)
	}
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "room-collaboration", "SKILL.md")); err != nil {
		t.Fatalf("系统托管 room-collaboration skill 未部署: %v", err)
	}
	claudeSkillLink := filepath.Join(agentValue.WorkspacePath, ".claude", "skills", "room-collaboration")
	if info, statErr := os.Lstat(claudeSkillLink); statErr != nil {
		t.Fatalf("room-collaboration skill 的 Claude 链接未生成: %v", statErr)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("room-collaboration skill 的 Claude 入口应为符号链接: %s", claudeSkillLink)
	}

	updated, err := workspaceService.UpdateFile(ctx, agentValue.AgentID, "notes/todo.md", "hello workspace")
	if err != nil {
		t.Fatalf("更新文件失败: %v", err)
	}
	if updated.Path != "notes/todo.md" {
		t.Fatalf("文件路径不正确: %+v", updated)
	}

	readBack, err := workspaceService.GetFile(ctx, agentValue.AgentID, "notes/todo.md")
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if readBack.Content != "hello workspace" {
		t.Fatalf("文件内容不匹配: %+v", readBack)
	}

	if _, err = workspaceService.CreateEntry(ctx, agentValue.AgentID, "docs", "directory", ""); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	renamed, err := workspaceService.RenameEntry(ctx, agentValue.AgentID, "notes/todo.md", "docs/todo.md")
	if err != nil {
		t.Fatalf("重命名文件失败: %v", err)
	}
	if renamed.NewPath != "docs/todo.md" {
		t.Fatalf("重命名结果不正确: %+v", renamed)
	}

	if _, err = workspaceService.DeleteEntry(ctx, agentValue.AgentID, "docs/todo.md"); err != nil {
		t.Fatalf("删除文件失败: %v", err)
	}
	if _, err = workspaceService.GetFile(ctx, agentValue.AgentID, "docs/todo.md"); err == nil {
		t.Fatal("删除后仍能读取文件")
	}

	if _, err = workspaceService.UpdateFile(ctx, agentValue.AgentID, ".agents/forbidden.txt", "x"); err == nil {
		t.Fatal("不应允许直接写入内部运行时目录")
	}
}

func TestServicePublishesWorkspaceLiveEvents(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "工作区实时助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	events := make(chan LiveEvent, 16)
	token, err := workspaceService.SubscribeLive(ctx, agentValue.AgentID, func(event LiveEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("订阅 workspace live 失败: %v", err)
	}
	defer workspaceService.UnsubscribeLive(token)
	time.Sleep(200 * time.Millisecond)

	if _, err = workspaceService.UpdateFile(ctx, agentValue.AgentID, "notes/live.md", "hello live"); err != nil {
		t.Fatalf("通过 API 更新文件失败: %v", err)
	}

	apiEvent := waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/live.md" &&
			event.Type == LiveEventFileWriteEnd &&
			event.Source == LiveSourceAPI
	})
	if apiEvent.ContentSnapshot == nil || *apiEvent.ContentSnapshot != "hello live" {
		t.Fatalf("API live 事件内容不正确: %+v", apiEvent)
	}

	agentFilePath := filepath.Join(agentValue.WorkspacePath, "notes", "agent.txt")
	if err = os.MkdirAll(filepath.Dir(agentFilePath), 0o755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	if err = os.WriteFile(agentFilePath, []byte("agent warmup"), 0o644); err != nil {
		t.Fatalf("模拟 agent 预热写文件失败: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if err = os.WriteFile(agentFilePath, []byte("agent write"), 0o644); err != nil {
		t.Fatalf("模拟 agent 写文件失败: %v", err)
	}

	agentEvent := waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/agent.txt" &&
			event.Type == LiveEventFileWriteEnd &&
			event.Source == LiveSourceAgent
	})
	if agentEvent.ContentSnapshot == nil || *agentEvent.ContentSnapshot != "agent write" {
		t.Fatalf("Agent live 事件内容不正确: %+v", agentEvent)
	}
}

func containsWorkspacePath(items []FileEntry, target string) bool {
	for _, item := range items {
		if item.Path == target {
			return true
		}
	}
	return false
}

func waitWorkspaceLiveEvent(t *testing.T, events <-chan LiveEvent, match func(LiveEvent) bool) LiveEvent {
	t.Helper()

	timeout := time.NewTimer(6 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case event := <-events:
			if match(event) {
				return event
			}
		case <-timeout.C:
			t.Fatal("等待 workspace live 事件超时")
		}
	}
}

func newWorkspaceTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	return config.Config{
		Host:                      "127.0.0.1",
		Port:                      18011,
		ProjectName:               "nexus-workspace-test",
		APIPrefix:                 "/nexus/v1",
		WebSocketPath:             "/nexus/v1/chat/ws",
		DefaultAgentID:            "nexus",
		WorkspacePath:             filepath.Join(root, "workspace"),
		CacheFileDir:              filepath.Join(root, "cache"),
		DatabaseDriver:            "sqlite",
		DatabaseURL:               filepath.Join(root, "nexus.db"),
		ConnectorOAuthRedirectURI: "http://localhost:3000/capability/connectors",
	}
}

func migrateWorkspaceSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, workspaceTestMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func workspaceTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
