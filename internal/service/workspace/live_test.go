package workspace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestServicePublishesWorkspaceLiveEvents(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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

func TestServiceFlushesWorkspaceLiveWrites(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "写入结算助手"})
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

	agentFilePath := filepath.Join(agentValue.WorkspacePath, "notes", "flush.txt")
	if err = os.MkdirAll(filepath.Dir(agentFilePath), 0o755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	if err = os.WriteFile(agentFilePath, []byte("flush warmup"), 0o644); err != nil {
		t.Fatalf("模拟 agent 预热写文件失败: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if err = os.WriteFile(agentFilePath, []byte("flush now"), 0o644); err != nil {
		t.Fatalf("模拟 agent 写文件失败: %v", err)
	}
	_ = waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/flush.txt" &&
			event.Type == LiveEventFileWriteDelta &&
			event.Source == LiveSourceAgent &&
			event.ContentSnapshot != nil &&
			*event.ContentSnapshot == "flush now"
	})

	workspaceService.FlushLiveWrites(agentValue.AgentID)
	flushedEvent := waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/flush.txt" &&
			event.Type == LiveEventFileWriteEnd &&
			event.Source == LiveSourceAgent
	})
	if flushedEvent.ContentSnapshot == nil || *flushedEvent.ContentSnapshot != "flush now" {
		t.Fatalf("强制结算 live 事件内容不正确: %+v", flushedEvent)
	}
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
