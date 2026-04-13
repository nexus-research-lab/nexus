// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：router_test.go
// @Date   ：2026/04/11 22:18:00
// @Author ：leemysw
// 2026/04/11 22:18:00   Create
// =====================================================

package channels

import (
	"context"
	"database/sql"
	"fmt"
	agentsvc "github.com/nexus-research-lab/nexus-core/internal/agent"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus-core/internal/permission"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	"github.com/nexus-research-lab/nexus-core/internal/sessiondomain"
	workspacestore "github.com/nexus-research-lab/nexus-core/internal/storage/workspace"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type stubAgentResolver struct {
	agentByID map[string]*agentsvc.Agent
}

func (r *stubAgentResolver) GetAgent(_ context.Context, agentID string) (*agentsvc.Agent, error) {
	item := r.agentByID[strings.TrimSpace(agentID)]
	if item == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	return item, nil
}

type stubPermissionSender struct {
	key    string
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (s *stubPermissionSender) Key() string {
	return s.key
}

func (s *stubPermissionSender) IsClosed() bool {
	return false
}

func (s *stubPermissionSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *stubPermissionSender) Events() []protocol.EventMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]protocol.EventMessage, len(s.events))
	copy(result, s.events)
	return result
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRouterDeliverTextUsesRememberedWebSocketRoute(t *testing.T) {
	workspacePath := t.TempDir()
	db := newChannelTestDB(t)
	permission := permissionctx.NewContext()
	resolver := &stubAgentResolver{
		agentByID: map[string]*agentsvc.Agent{
			"agent-1": {
				AgentID:       "agent-1",
				WorkspacePath: workspacePath,
			},
		},
	}
	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "chat-1", "")
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, sessiondomain.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-1",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Test",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("创建测试 session 失败: %v", err)
	}

	router := NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		resolver,
		permission,
	)
	sender := &stubPermissionSender{key: "sender-1"}
	permission.BindSession(sessionKey, sender, "client-1", true)

	if err := router.RememberWebSocketRoute(context.Background(), sessionKey); err != nil {
		t.Fatalf("RememberWebSocketRoute 失败: %v", err)
	}
	target, err := router.DeliverText(context.Background(), "agent-1", "自动提醒", DeliveryTarget{Mode: DeliveryModeLast})
	if err != nil {
		t.Fatalf("DeliverText 失败: %v", err)
	}
	if target.Channel != ChannelTypeWebSocket || target.To != sessionKey {
		t.Fatalf("解析后的投递目标不正确: %+v", target)
	}

	messages, err := store.ReadSessionMessages([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取消息失败: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("期望写入 2 条消息，实际 %d", len(messages))
	}
	if stringValue(messages[0]["role"]) != "assistant" || stringValue(messages[1]["role"]) != "result" {
		t.Fatalf("投递消息角色顺序不正确: %+v", messages)
	}
	if stringValue(messages[1]["result"]) != "自动提醒" {
		t.Fatalf("result 内容不正确: %+v", messages[1])
	}

	events := sender.Events()
	if len(events) != 2 {
		t.Fatalf("期望广播 2 条 durable message，实际 %d", len(events))
	}
	if events[0].EventType != protocol.EventTypeMessage || events[1].EventType != protocol.EventTypeMessage {
		t.Fatalf("广播事件类型不正确: %+v", events)
	}
}

func TestDiscordChannelSendDeliveryText(t *testing.T) {
	requests := make([]*http.Request, 0)
	channel := newDiscordChannel("token-1", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://discord.test/api/v10"

	text := strings.Repeat("a", 2400)
	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeDiscord,
		To:      "123456",
	}, text); err != nil {
		t.Fatalf("Discord 发送失败: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("期望分片发送 2 次，实际 %d", len(requests))
	}
	if got := requests[0].Header.Get("Authorization"); got != "Bot token-1" {
		t.Fatalf("Authorization 头不正确: %s", got)
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/channels/123456/messages") {
		t.Fatalf("Discord 路径不正确: %s", requests[0].URL.Path)
	}
}

func TestTelegramChannelSendDeliveryText(t *testing.T) {
	requests := make([]*http.Request, 0)
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://telegram.test"

	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, "hello"); err != nil {
		t.Fatalf("Telegram 发送失败: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("期望发送 1 次，实际 %d", len(requests))
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/bottoken-2/sendMessage") {
		t.Fatalf("Telegram 路径不正确: %s", requests[0].URL.Path)
	}
}

func newChannelTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := `
CREATE TABLE automation_delivery_routes (
    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    channel VARCHAR(64),
    "to" VARCHAR(255),
    account_id VARCHAR(64),
    thread_id VARCHAR(255),
    enabled BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);`
	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("初始化 delivery schema 失败: %v", err)
	}
	return db
}
