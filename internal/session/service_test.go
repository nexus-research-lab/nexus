// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service_test.go
// @Date   ：2026/04/11 00:26:00
// @Author ：leemysw
// 2026/04/11 00:26:00   Create
// =====================================================

package session_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/session"
	workspace2 "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestSessionServiceLifecycle(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := bootstrap.NewSessionServiceWithDB(cfg, db, agentService)
	sessionService.SetRuntimeManager(runtimectx.NewManager())

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, agent2.CreateRequest{Name: "测试会话助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "launcher-app-"+agentA.AgentID, "")
	created, err := sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: dmKey,
		Title:      "Launcher App",
	})
	if err != nil {
		t.Fatalf("创建普通 session 失败: %v", err)
	}
	if created.Title != "Launcher App" {
		t.Fatalf("session 标题不正确: got=%s", created.Title)
	}

	seedWorkspaceSessionArtifacts(t, cfg, agentA.AgentID, agentA.WorkspacePath, dmKey)

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}
	seedRoomConversationMessages(t, cfg, dmContext.Conversation.ID)

	sessions, err := sessionService.ListSessions(ctx)
	if err != nil {
		t.Fatalf("列出 sessions 失败: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("session 列表未合并 room 视图: got=%d", len(sessions))
	}

	agentSessions, err := sessionService.ListAgentSessions(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取 agent sessions 失败: %v", err)
	}
	if len(agentSessions) < 2 {
		t.Fatalf("agent sessions 数量不正确: got=%d", len(agentSessions))
	}

	messages, err := sessionService.GetSessionMessages(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取普通 session 消息失败: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("消息归一化结果不正确: got=%d want=3", len(messages))
	}
	if messages[1]["content"] != "最终回复" {
		t.Fatalf("消息压缩未保留最新快照: %+v", messages[1])
	}
	if messages[1]["stream_status"] != "cancelled" {
		t.Fatalf("未终止 round 的 assistant 应归一化为 cancelled: %+v", messages[1])
	}
	if messages[2]["role"] != "result" || messages[2]["subtype"] != "interrupted" {
		t.Fatalf("未终止 round 未物化 interrupted result: %+v", messages)
	}

	roomMessages, err := sessionService.GetSessionMessages(ctx, protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID))
	if err != nil {
		t.Fatalf("读取 Room 共享流失败: %v", err)
	}
	if len(roomMessages) != 2 {
		t.Fatalf("Room 共享消息数量不正确: got=%d want=2", len(roomMessages))
	}
	if roomMessages[0]["stream_status"] != "cancelled" {
		t.Fatalf("Room assistant 快照未归一化为 cancelled: %+v", roomMessages[0])
	}
	if roomMessages[1]["role"] != "result" || roomMessages[1]["subtype"] != "interrupted" {
		t.Fatalf("Room 未终止 round 未物化 interrupted result: %+v", roomMessages)
	}

	cost, err := sessionService.GetSessionCostSummary(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取 session 成本失败: %v", err)
	}
	if cost.TotalTokens != 42 || cost.CompletedRounds != 1 {
		t.Fatalf("session 成本汇总不正确: %+v", cost)
	}

	agentCost, err := sessionService.GetAgentCostSummary(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取 agent 成本失败: %v", err)
	}
	if agentCost.TotalTokens != 42 || agentCost.CostSessions != 1 {
		t.Fatalf("agent 成本汇总不正确: %+v", agentCost)
	}

	updatedTitle := "Launcher 重命名"
	updated, err := sessionService.UpdateSession(ctx, dmKey, sessionsvc.UpdateRequest{Title: &updatedTitle})
	if err != nil {
		t.Fatalf("更新 session 失败: %v", err)
	}
	if updated.Title != updatedTitle {
		t.Fatalf("更新标题失败: got=%s want=%s", updated.Title, updatedTitle)
	}

	if err = sessionService.DeleteSession(ctx, dmKey); err != nil {
		t.Fatalf("删除 session 失败: %v", err)
	}
	if _, err = sessionService.GetSession(ctx, dmKey); err == nil {
		t.Fatal("删除后不应还能读取到 session")
	}
}

func TestSessionServiceGetSessionMessagesSkipsActiveRoundMaterialization(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := bootstrap.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, agent2.CreateRequest{Name: "活跃轮次助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "active-"+agentA.AgentID, "")
	if _, err = sessionService.CreateSession(ctx, sessionsvc.CreateRequest{SessionKey: dmKey}); err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}
	seedWorkspaceSessionArtifacts(t, cfg, agentA.AgentID, agentA.WorkspacePath, dmKey)
	runtimeManager.StartRound(dmKey, "round_1", nil)
	defer runtimeManager.MarkRoundFinished(dmKey, "round_1")

	messages, err := sessionService.GetSessionMessages(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取 session 消息失败: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("活跃 round 不应物化 interrupted result: got=%d want=2", len(messages))
	}
	if _, exists := messages[1]["stream_status"]; exists {
		t.Fatalf("活跃 round 不应把 assistant 快照强制终止: %+v", messages[1])
	}
}

func seedWorkspaceSessionArtifacts(t *testing.T, cfg config.Config, agentID string, workspacePath string, sessionKey string) {
	t.Helper()

	store := workspace2.NewSessionFileStore(cfg.WorkspacePath)
	messagePath := workspace2.New(cfg.WorkspacePath).SessionMessagePath(workspacePath, sessionKey)
	costPath := workspace2.New(cfg.WorkspacePath).SessionCostPath(workspacePath, sessionKey)

	rows := []map[string]any{
		{
			"message_id":  "msg_user_1",
			"session_key": sessionKey,
			"agent_id":    agentID,
			"round_id":    "round_1",
			"role":        "user",
			"content":     "你好",
			"timestamp":   1000,
		},
		{
			"message_id":  "msg_assistant_1",
			"session_key": sessionKey,
			"agent_id":    agentID,
			"round_id":    "round_1",
			"role":        "assistant",
			"content":     "草稿回复",
			"timestamp":   2000,
		},
		{
			"message_id":  "msg_assistant_1",
			"session_key": sessionKey,
			"agent_id":    agentID,
			"round_id":    "round_1",
			"role":        "assistant",
			"content":     "最终回复",
			"timestamp":   3000,
		},
	}
	writeJSONL(t, messagePath, rows)

	costRows := []map[string]any{
		{
			"agent_id":                    agentID,
			"session_key":                 sessionKey,
			"session_id":                  "sdk_1",
			"round_id":                    "round_1",
			"subtype":                     "success",
			"input_tokens":                10,
			"output_tokens":               32,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     0,
			"total_cost_usd":              0.12,
			"duration_ms":                 900,
			"message_id":                  "msg_result_1",
		},
	}
	writeJSONL(t, costPath, costRows)

	_, err := store.ReadSessionCostSummary([]string{workspacePath}, sessionKey, agentID)
	if err != nil {
		t.Fatalf("预热 session 成本汇总失败: %v", err)
	}
}

func seedRoomConversationMessages(t *testing.T, cfg config.Config, conversationID string) {
	t.Helper()

	store := workspace2.New(cfg.WorkspacePath)
	messagePath := store.RoomConversationMessagePath(conversationID)
	rows := []map[string]any{
		{
			"message_id":      "room_msg_1",
			"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
			"conversation_id": conversationID,
			"agent_id":        "agent_room",
			"round_id":        "room_round_1",
			"role":            "assistant",
			"content":         "Room 共享消息",
			"timestamp":       100,
		},
	}
	writeJSONL(t, messagePath, rows)
}

func writeJSONL(t *testing.T, path string, rows []map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err = encoder.Encode(row); err != nil {
			t.Fatalf("写入 jsonl 失败: %v", err)
		}
	}
}

func newSessionTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18012,
		ProjectName:    "nexus-session-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateSessionSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, sessionMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func sessionMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
