// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：history_migrator_test.go
// @Date   ：2026/04/19 16:10:00
// @Author ：leemysw
// 2026/04/19 16:10:00   Create
// =====================================================

package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentmodel "github.com/nexus-research-lab/nexus/internal/model/agent"
	roommodel "github.com/nexus-research-lab/nexus/internal/model/room"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestHistoryMigratorMigratesLegacyAgentSessionToOverlay(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	history := NewAgentHistoryStore(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	sessionKey := protocol.BuildAgentSessionKey("agent-a", "ws", "dm", "legacy-chat", "")
	sessionValue := sessionmodel.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-a",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    time.Unix(0, 0).UTC(),
		LastActivity: time.Unix(0, 0).UTC(),
		Title:        "Legacy Chat",
		Options:      map[string]any{},
		IsActive:     true,
	}
	if _, err := files.UpsertSession(workspacePath, sessionValue); err != nil {
		t.Fatalf("写入 session meta 失败: %v", err)
	}
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "round_1",
		"round_id":   "round_1",
		"role":       "user",
		"content":    "你好",
		"timestamp":  1000,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "assistant_1",
		"round_id":   "round_1",
		"role":       "assistant",
		"content":    "你好，这是一条旧历史回复。",
		"timestamp":  1001,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "result_1",
		"round_id":   "round_1",
		"role":       "result",
		"subtype":    "success",
		"result":     "done",
		"timestamp":  1002,
	})

	change, err := migrator.MigrateAgentSession(workspacePath, sessionValue)
	if err != nil {
		t.Fatalf("迁移 agent session 失败: %v", err)
	}
	if !change.Migrated || !change.RemovedLegacyFile {
		t.Fatalf("迁移结果不正确: %+v", change)
	}
	if _, err := os.Stat(paths.SessionMessagePath(workspacePath, sessionKey)); !os.IsNotExist(err) {
		t.Fatalf("旧 messages.jsonl 应已删除: %v", err)
	}

	meta, _, err := files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("重新读取 session meta 失败: %v", err)
	}
	if meta == nil || !sessionmodel.IsTranscriptHistory(meta.Options) {
		t.Fatalf("session meta 未切到 transcript: %+v", meta)
	}

	overlayRows, err := files.readJSONL(paths.SessionOverlayPath(workspacePath, sessionKey))
	if err != nil {
		t.Fatalf("读取 overlay 失败: %v", err)
	}
	if len(overlayRows) != 3 {
		t.Fatalf("overlay 行数不正确: got=%d want=3 rows=%+v", len(overlayRows), overlayRows)
	}

	rows, err := history.ReadMessages(workspacePath, *meta, nil)
	if err != nil {
		t.Fatalf("读取迁移后的历史失败: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("迁移后历史条数不正确: got=%d want=3 rows=%+v", len(rows), rows)
	}
	if role := strings.TrimSpace(stringFromAny(rows[0]["role"])); role != "user" {
		t.Fatalf("首条消息应为用户消息: %+v", rows)
	}
}

func TestHistoryMigratorMovesCompactSessionDirToCurrentLayout(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	sessionKey := protocol.BuildAgentSessionKey("agent-a", "ws", "dm", "legacy-chat", "")
	sessionValue := sessionmodel.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-a",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    time.Unix(0, 0).UTC(),
		LastActivity: time.Unix(0, 0).UTC(),
		Title:        "Legacy Chat",
		Options:      map[string]any{},
		IsActive:     true,
	}

	legacyDir := paths.CompactSessionDir(workspacePath, sessionKey)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("创建 compact session 目录失败: %v", err)
	}
	if err := files.replaceJSONL(filepath.Join(legacyDir, "messages.jsonl"), []map[string]any{
		{
			"message_id": "round_1",
			"round_id":   "round_1",
			"role":       "user",
			"content":    "你好",
			"timestamp":  1000,
		},
	}); err != nil {
		t.Fatalf("写入 compact legacy 消息失败: %v", err)
	}

	change, err := migrator.MigrateAgentSession(workspacePath, sessionValue)
	if err != nil {
		t.Fatalf("迁移 compact session 目录失败: %v", err)
	}
	if !change.MovedLegacyDir {
		t.Fatalf("未标记迁移旧目录: %+v", change)
	}
	if _, err := os.Stat(paths.SessionDir(workspacePath, sessionKey)); err != nil {
		t.Fatalf("新目录不存在: %v", err)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("旧 compact 目录未被清理: %v", err)
	}
}

func TestHistoryMigratorSkipsTranscriptBackedDurableRows(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	history := NewAgentHistoryStore(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	sessionKey := protocol.BuildAgentSessionKey("agent-a", "ws", "dm", "transcript-chat", "")
	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	sessionValue := sessionmodel.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-a",
		SessionID:    stringPointer(sessionID),
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    time.Unix(0, 0).UTC(),
		LastActivity: time.Unix(0, 0).UTC(),
		Title:        "Transcript Chat",
		Options:      map[string]any{},
		IsActive:     true,
	}
	if _, err := files.UpsertSession(workspacePath, sessionValue); err != nil {
		t.Fatalf("写入 session meta 失败: %v", err)
	}
	writeHistoryTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-19T12:00:00Z",
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "msg_assistant_1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "这是 transcript 回复。"},
				},
			},
		},
		{
			"type":            "result",
			"uuid":            "msg_result_1",
			"session_id":      sessionID,
			"parentUuid":      "msg_assistant_1",
			"subtype":         "success",
			"duration_ms":     12,
			"duration_api_ms": 8,
			"num_turns":       1,
			"result":          "done",
			"is_error":        false,
		},
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "round_1",
		"round_id":   "round_1",
		"role":       "user",
		"content":    "你好",
		"timestamp":  1000,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "msg_assistant_1",
		"round_id":   "round_1",
		"role":       "assistant",
		"content":    "这是 transcript 回复。",
		"timestamp":  1001,
		"session_id": sessionID,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "msg_result_1",
		"round_id":   "round_1",
		"role":       "result",
		"subtype":    "success",
		"result":     "done",
		"timestamp":  1002,
		"session_id": sessionID,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "synthetic_result_1",
		"round_id":   "round_1",
		"role":       "result",
		"subtype":    "interrupted",
		"result":     "外部补发结果",
		"timestamp":  1003,
	})

	change, err := migrator.MigrateAgentSession(workspacePath, sessionValue)
	if err != nil {
		t.Fatalf("迁移 transcript session 失败: %v", err)
	}
	if change.WrittenOverlayRows != 2 || change.WrittenRoundMarkers != 1 {
		t.Fatalf("迁移统计不正确: %+v", change)
	}

	overlayRows, err := files.readJSONL(paths.SessionOverlayPath(workspacePath, sessionKey))
	if err != nil {
		t.Fatalf("读取 overlay 失败: %v", err)
	}
	if len(overlayRows) != 3 {
		t.Fatalf("overlay 行数不正确: got=%d want=3 rows=%+v", len(overlayRows), overlayRows)
	}
	for _, row := range overlayRows {
		if messageID := strings.TrimSpace(stringFromAny(row["message_id"])); messageID == "msg_assistant_1" {
			t.Fatalf("transcript 已覆盖的 durable 消息不应重复写入 overlay: %+v", overlayRows)
		}
	}

	meta, _, err := files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("重新读取 session meta 失败: %v", err)
	}
	rows, err := history.ReadMessages(workspacePath, *meta, nil)
	if err != nil {
		t.Fatalf("读取迁移后的 transcript 历史失败: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("迁移后 transcript 历史条数不正确: got=%d want=4 rows=%+v", len(rows), rows)
	}
}

func TestAgentHistoryStoreProjectsCompletedAssistantSnapshotFromTranscript(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	history := NewAgentHistoryStore(workspaceRoot)

	sessionKey := protocol.BuildAgentSessionKey("agent-a", "ws", "dm", "leaf-assistant", "")
	sessionID := "2944aa53-db7c-4b9f-a3e6-74401402abc5"
	sessionValue := sessionmodel.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-a",
		SessionID:    stringPointer(sessionID),
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    time.Unix(0, 0).UTC(),
		LastActivity: time.Unix(0, 0).UTC(),
		Title:        "Leaf Assistant",
		Options: map[string]any{
			sessionmodel.OptionHistorySource: sessionmodel.HistorySourceTranscript,
		},
		IsActive:     true,
	}
	if _, err := files.UpsertSession(workspacePath, sessionValue); err != nil {
		t.Fatalf("写入 session meta 失败: %v", err)
	}
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round_1", "啥意思", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round_2", "这才是最新问题", 2000); err != nil {
		t.Fatalf("写入第二条 round marker 失败: %v", err)
	}
	writeHistoryTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "queue-operation",
			"uuid":      "queue-enqueue-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-19T12:12:20Z",
			"parentUuid": "",
		},
		{
			"type":      "user",
			"uuid":      "user_1",
			"sessionId": sessionID,
			"timestamp": "2026-04-19T12:12:21Z",
			"message": map[string]any{
				"role": "user",
				"content": "啥意思",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant_thinking_1",
			"sessionId":  sessionID,
			"timestamp":  "2026-04-19T12:12:22Z",
			"parentUuid": "user_1",
			"message": map[string]any{
				"id":   "msg_1",
				"role": "assistant",
				"usage": map[string]any{
					"input_tokens":  0,
					"output_tokens": 0,
				},
				"content": []map[string]any{
					{"type": "thinking", "thinking": "先解释用户的问题"},
				},
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant_text_1",
			"sessionId":  sessionID,
			"timestamp":  "2026-04-19T12:12:23Z",
			"parentUuid": "assistant_thinking_1",
			"message": map[string]any{
				"id":   "msg_1",
				"role": "assistant",
				"stop_reason": "end_turn",
				"usage": map[string]any{
					"input_tokens":                83,
					"cache_creation_input_tokens": 0,
					"cache_read_input_tokens":     16695,
					"output_tokens":               108,
					"server_tool_use": map[string]any{
						"web_search_requests": 0,
						"web_fetch_requests":  0,
					},
					"service_tier": "standard",
					"cache_creation": map[string]any{
						"ephemeral_1h_input_tokens": 0,
						"ephemeral_5m_input_tokens": 0,
					},
					"inference_geo": "",
					"iterations":    []any{},
					"speed":         "standard",
				},
				"content": []map[string]any{
					{"type": "text", "text": "你好！请给我更多上下文。"},
				},
			},
		},
		{
			"type":       "result",
			"uuid":       "transcript_result_1",
			"sessionId":  sessionID,
			"timestamp":  "2026-04-19T12:12:24Z",
			"parentUuid": "assistant_text_1",
			"subtype":    "success",
			"duration_ms": 19,
			"result":     "这条 transcript result 不应进入历史",
		},
	})

	rows, err := history.ReadMessages(workspacePath, sessionValue, nil)
	if err != nil {
		t.Fatalf("读取 transcript 历史失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("历史条数不正确: got=%d want=2 rows=%+v", len(rows), rows)
	}
	if got := strings.TrimSpace(stringFromAny(rows[0]["content"])); got != "这才是最新问题" {
		t.Fatalf("transcript user 应绑定最新 round marker: got=%s rows=%+v", got, rows)
	}
	if role := strings.TrimSpace(stringFromAny(rows[1]["role"])); role != "assistant" {
		t.Fatalf("第二条应为 assistant: %+v", rows)
	}
	if rows[1]["is_complete"] != true {
		t.Fatalf("带 stop_reason 的 assistant 快照应视为已完成: %+v", rows[1])
	}
	if status := strings.TrimSpace(stringFromAny(rows[1]["stream_status"])); status != "done" {
		t.Fatalf("带 stop_reason 的 assistant 快照应归一化为 done: %+v", rows[1])
	}
	usage, ok := rows[1]["usage"].(map[string]any)
	if !ok {
		t.Fatalf("assistant usage 应从 transcript 带回: %+v", rows[1])
	}
	if got := intFromAny(usage["cache_read_input_tokens"]); got != 16695 {
		t.Fatalf("cache_read_input_tokens 不正确: got=%d usage=%+v", got, usage)
	}
	serverToolUse, ok := usage["server_tool_use"].(map[string]any)
	if !ok {
		t.Fatalf("server_tool_use 应保留: %+v", usage)
	}
	if got := intFromAny(serverToolUse["web_fetch_requests"]); got != 0 {
		t.Fatalf("server_tool_use.web_fetch_requests 不正确: got=%d usage=%+v", got, usage)
	}
	if got := strings.TrimSpace(stringFromAny(usage["service_tier"])); got != "standard" {
		t.Fatalf("service_tier 不正确: got=%s usage=%+v", got, usage)
	}
	if got := strings.TrimSpace(stringFromAny(usage["speed"])); got != "standard" {
		t.Fatalf("speed 不正确: got=%s usage=%+v", got, usage)
	}
}

func TestHistoryMigratorRemovesInvalidLegacyAgentRows(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	history := NewAgentHistoryStore(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	sessionKey := protocol.BuildAgentSessionKey("agent-a", "ws", "dm", "legacy-invalid", "")
	sessionValue := sessionmodel.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-a",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    time.Unix(0, 0).UTC(),
		LastActivity: time.Unix(0, 0).UTC(),
		Title:        "Legacy Invalid",
		Options:      map[string]any{},
		IsActive:     true,
	}
	if _, err := files.UpsertSession(workspacePath, sessionValue); err != nil {
		t.Fatalf("写入 session meta 失败: %v", err)
	}
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "round_1",
		"round_id":   "round_1",
		"role":       "user",
		"content":    "你好",
		"timestamp":  1000,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "invalid_missing_role",
		"round_id":   "round_1",
		"content":    "这条没有 role",
		"timestamp":  1001,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "invalid_empty_user",
		"round_id":   "round_2",
		"role":       "user",
		"content":    "",
		"timestamp":  1002,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "invalid_empty_assistant",
		"round_id":   "round_2",
		"role":       "assistant",
		"timestamp":  1003,
	})
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "result_1",
		"round_id":   "round_1",
		"role":       "result",
		"subtype":    "success",
		"result":     "done",
		"timestamp":  1004,
	})

	change, err := migrator.MigrateAgentSession(workspacePath, sessionValue)
	if err != nil {
		t.Fatalf("迁移带坏行的 agent session 失败: %v", err)
	}
	if change.RemovedInvalidRows != 3 {
		t.Fatalf("无效行移除统计不正确: %+v", change)
	}

	overlayRows, err := files.readJSONL(paths.SessionOverlayPath(workspacePath, sessionKey))
	if err != nil {
		t.Fatalf("读取 overlay 失败: %v", err)
	}
	if len(overlayRows) != 2 {
		t.Fatalf("清洗后 overlay 行数不正确: got=%d want=2 rows=%+v", len(overlayRows), overlayRows)
	}

	meta, _, err := files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("重新读取 session meta 失败: %v", err)
	}
	rows, err := history.ReadMessages(workspacePath, *meta, nil)
	if err != nil {
		t.Fatalf("读取迁移后的历史失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("清洗后历史条数不正确: got=%d want=2 rows=%+v", len(rows), rows)
	}
}

func TestHistoryMigratorMigratesRoomConversationToRefs(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	roomHistory := NewRoomHistoryStore(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	conversationID := "conversation-1"
	roomID := "room-1"
	agentID := "agent-a"
	sessionID := "660e8400-e29b-41d4-a716-446655440000"
	privateSessionKey := protocol.BuildRoomAgentSessionKey(conversationID, agentID, roommodel.RoomTypeGroup)
	writeHistoryTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-19T12:00:00Z",
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "msg_assistant_1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Room transcript 回复。"},
				},
			},
		},
		{
			"type":            "result",
			"uuid":            "msg_result_1",
			"session_id":      sessionID,
			"parentUuid":      "msg_assistant_1",
			"subtype":         "success",
			"duration_ms":     12,
			"duration_api_ms": 8,
			"num_turns":       1,
			"result":          "done",
			"is_error":        false,
		},
	})
	if err := NewAgentHistoryStore(workspaceRoot).AppendRoundMarker(workspacePath, privateSessionKey, "room_round_1", "你好", 1000); err != nil {
		t.Fatalf("写入 room round marker 失败: %v", err)
	}
	if transcriptPath, err := NewAgentHistoryStore(workspaceRoot).resolveTranscriptPath(workspacePath, sessionID); err != nil {
		t.Fatalf("resolve transcript path 失败: %v", err)
	} else if strings.TrimSpace(transcriptPath) == "" {
		t.Fatal("resolve transcript path 返回空字符串")
	}

	legacyRoomRows := []map[string]any{
		{
			"message_id":      "room_round_1",
			"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
			"room_id":         roomID,
			"conversation_id": conversationID,
			"round_id":        "room_round_1",
			"role":            "user",
			"content":         "你好",
			"timestamp":       1000,
		},
		{
			"message_id":      "msg_assistant_1",
			"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
			"room_id":         roomID,
			"conversation_id": conversationID,
			"agent_id":        agentID,
			"round_id":        "room_round_1",
			"role":            "assistant",
			"timestamp":       1001,
		},
		{
			"message_id":      "msg_result_1",
			"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
			"room_id":         roomID,
			"conversation_id": conversationID,
			"agent_id":        agentID,
			"round_id":        "room_round_1",
			"role":            "result",
			"subtype":         "success",
			"timestamp":       1002,
		},
		{
			"message_id":      "room_hint_1",
			"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
			"room_id":         roomID,
			"conversation_id": conversationID,
			"round_id":        "room_round_1",
			"role":            "result",
			"subtype":         "error",
			"result":          "请使用 @AgentName 指定要对话的成员",
			"timestamp":       1003,
		},
	}
	if err := files.replaceJSONL(paths.LegacyRoomConversationMessagePath(conversationID), legacyRoomRows); err != nil {
		t.Fatalf("写入旧 room 共享历史失败: %v", err)
	}

	contextValue := roommodel.ConversationContextAggregate{
		Room: roommodel.RoomRecord{
			ID:       roomID,
			RoomType: roommodel.RoomTypeGroup,
		},
		MemberAgents: []agentmodel.Agent{
			{
				AgentID:       agentID,
				WorkspacePath: workspacePath,
			},
		},
		Conversation: roommodel.ConversationRecord{
			ID:     conversationID,
			RoomID: roomID,
		},
		Sessions: []roommodel.SessionRecord{
			{
				ConversationID: conversationID,
				AgentID:        agentID,
				IsPrimary:      true,
				SDKSessionID:   sessionID,
			},
		},
	}
	sources, err := migrator.buildRoomTranscriptSources(contextValue)
	if err != nil {
		t.Fatalf("构建 room transcript source 失败: %v", err)
	}
	source, exists := sources[agentID]
	if !exists {
		t.Fatalf("未找到 room transcript source: %+v", sources)
	}
	if _, exists = source.MessageIndexByID["msg_assistant_1"]; !exists {
		t.Fatalf("assistant transcript 索引缺失: %+v", source.MessageIndexByID)
	}
	change, err := migrator.MigrateRoomConversation(contextValue)
	if err != nil {
		t.Fatalf("迁移 room 共享历史失败: %v", err)
	}
	overlayRows, err := files.readJSONL(paths.RoomConversationOverlayPath(conversationID))
	if err != nil {
		t.Fatalf("读取 room overlay 失败: %v", err)
	}
	if change.WrittenRefs != 1 || change.WrittenOverlayRows != 3 {
		t.Fatalf("迁移统计不正确: change=%+v overlay=%+v", change, overlayRows)
	}
	if _, err := os.Stat(paths.LegacyRoomConversationMessagePath(conversationID)); !os.IsNotExist(err) {
		t.Fatalf("旧 room messages.jsonl 应已删除: %v", err)
	}
	if len(overlayRows) != 4 {
		t.Fatalf("room overlay 行数不正确: got=%d want=4 rows=%+v", len(overlayRows), overlayRows)
	}
	refCount := 0
	for _, row := range overlayRows {
		if strings.TrimSpace(stringFromAny(row[overlayKindField])) == overlayKindTranscriptRef {
			refCount++
		}
	}
	if refCount != 1 {
		t.Fatalf("room transcript_ref 数量不正确: got=%d want=1 rows=%+v", refCount, overlayRows)
	}

	rows, err := roomHistory.ReadMessages(conversationID, nil)
	if err != nil {
		t.Fatalf("读取迁移后的 room 历史失败: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("迁移后 room 历史条数不正确: got=%d want=4 rows=%+v", len(rows), rows)
	}
}

func TestHistoryMigratorMovesCompactRoomDirToCurrentLayout(t *testing.T) {
	workspaceRoot, _ := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	conversationID := "conversation-1"
	roomID := "room-1"
	legacyDir := paths.CompactRoomConversationDir(conversationID)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("创建 compact room 目录失败: %v", err)
	}
	if err := files.replaceJSONL(filepath.Join(legacyDir, "messages.jsonl"), []map[string]any{
		{
			"message_id":      "room_round_1",
			"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
			"room_id":         roomID,
			"conversation_id": conversationID,
			"round_id":        "room_round_1",
			"role":            "user",
			"content":         "你好",
			"timestamp":       1000,
		},
	}); err != nil {
		t.Fatalf("写入 compact room 消息失败: %v", err)
	}

	change, err := migrator.MigrateRoomConversation(roommodel.ConversationContextAggregate{
		Room: roommodel.RoomRecord{
			ID:       roomID,
			RoomType: roommodel.RoomTypeGroup,
		},
		Conversation: roommodel.ConversationRecord{
			ID:     conversationID,
			RoomID: roomID,
		},
	})
	if err != nil {
		t.Fatalf("迁移 compact room 目录失败: %v", err)
	}
	if !change.MovedLegacyDir {
		t.Fatalf("未标记迁移旧 room 目录: %+v", change)
	}
	if _, err := os.Stat(paths.RoomConversationDir(conversationID)); err != nil {
		t.Fatalf("新 room 目录不存在: %v", err)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("旧 compact room 目录未被清理: %v", err)
	}
}

func TestHistoryMigratorRemovesInvalidRoomOverlayRowsWithoutLegacyMessages(t *testing.T) {
	workspaceRoot, _ := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	conversationID := "conversation-invalid"
	roomID := "room-invalid"
	overlayPath := paths.RoomConversationOverlayPath(conversationID)
	if err := files.replaceJSONL(overlayPath, []map[string]any{
		{
			overlayKindField:  overlayKindTranscriptRef,
			"message_id":      "msg_missing_ref_fields",
			"conversation_id": conversationID,
			"room_id":         roomID,
			"agent_id":        "agent-a",
			"timestamp":       1000,
		},
		{
			"message_id":      "room_round_1",
			"conversation_id": conversationID,
			"room_id":         roomID,
			"round_id":        "room_round_1",
			"role":            "user",
			"content":         "你好",
			"timestamp":       1001,
		},
	}); err != nil {
		t.Fatalf("写入 room overlay 失败: %v", err)
	}

	change, err := migrator.MigrateRoomConversation(roommodel.ConversationContextAggregate{
		Room: roommodel.RoomRecord{
			ID:       roomID,
			RoomType: roommodel.RoomTypeGroup,
		},
		Conversation: roommodel.ConversationRecord{
			ID:     conversationID,
			RoomID: roomID,
		},
	})
	if err != nil {
		t.Fatalf("迁移仅清洗 room overlay 失败: %v", err)
	}
	if !change.Migrated || change.RemovedInvalidRows != 1 {
		t.Fatalf("room overlay 清洗统计不正确: %+v", change)
	}

	rows, err := files.readJSONL(overlayPath)
	if err != nil {
		t.Fatalf("读取清洗后的 room overlay 失败: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("清洗后的 room overlay 行数不正确: got=%d want=1 rows=%+v", len(rows), rows)
	}
}

func TestHistoryMigratorRemovesEmptyRoomConversationDir(t *testing.T) {
	workspaceRoot, _ := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	conversationID := "conversation-empty-room"
	roomID := "room-empty-room"
	overlayPath := paths.RoomConversationOverlayPath(conversationID)
	if err := files.replaceJSONL(overlayPath, []map[string]any{
		{
			overlayKindField:  overlayKindTranscriptRef,
			"message_id":      "msg_missing_ref_fields",
			"conversation_id": conversationID,
			"room_id":         roomID,
			"agent_id":        "agent-a",
			"timestamp":       1000,
		},
	}); err != nil {
		t.Fatalf("写入仅包含坏行的 room overlay 失败: %v", err)
	}

	change, err := migrator.MigrateRoomConversation(roommodel.ConversationContextAggregate{
		Room: roommodel.RoomRecord{
			ID:       roomID,
			RoomType: roommodel.RoomTypeGroup,
		},
		Conversation: roommodel.ConversationRecord{
			ID:     conversationID,
			RoomID: roomID,
		},
	})
	if err != nil {
		t.Fatalf("迁移空 room overlay 失败: %v", err)
	}
	if !change.Migrated || change.RemovedInvalidRows != 1 {
		t.Fatalf("空 room 目录清理统计不正确: %+v", change)
	}
	if _, err := os.Stat(paths.RoomConversationDir(conversationID)); !os.IsNotExist(err) {
		t.Fatalf("空 room 目录应已删除: %v", err)
	}
}

func TestHistoryMigratorRemovesEmptyRoomPrivateSessionDir(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	files := NewSessionFileStore(workspaceRoot)
	paths := New(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	conversationID := "conversation-room-private"
	roomID := "room-room-private"
	sessionKey := protocol.BuildRoomAgentSessionKey(conversationID, "agent-a", roommodel.RoomTypeGroup)
	if err := os.MkdirAll(paths.SessionDir(workspacePath, sessionKey), 0o755); err != nil {
		t.Fatalf("创建 room 私有 session 目录失败: %v", err)
	}
	appendLegacyMessage(t, files, workspacePath, sessionKey, sessionmodel.Message{
		"message_id": "invalid_room_private",
		"round_id":   "room_round_1",
		"role":       "assistant",
		"timestamp":  1000,
	})

	change, err := migrator.MigrateAgentSession(workspacePath, sessionmodel.Session{
		SessionKey:     sessionKey,
		AgentID:        "agent-a",
		ConversationID: stringPointer(conversationID),
		RoomID:         stringPointer(roomID),
		ChannelType:    "websocket",
		ChatType:       "group",
		Status:         "active",
		CreatedAt:      time.Unix(0, 0).UTC(),
		LastActivity:   time.Unix(0, 0).UTC(),
		Title:          "Room Private",
		Options:        map[string]any{},
		IsActive:       true,
	})
	if err != nil {
		t.Fatalf("迁移空 room 私有 session 失败: %v", err)
	}
	if !change.Migrated || !change.RemovedLegacyFile || change.RemovedInvalidRows != 1 {
		t.Fatalf("room 私有 session 清理统计不正确: %+v", change)
	}
	if _, err := os.Stat(paths.SessionDir(workspacePath, sessionKey)); !os.IsNotExist(err) {
		t.Fatalf("空 room 私有 session 目录应已删除: %v", err)
	}
}

func TestHistoryMigratorPrunesOrphanSessionDirs(t *testing.T) {
	workspaceRoot, workspacePath := newHistoryMigratorTestPaths(t)
	paths := New(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	validSessionKey := protocol.BuildAgentSessionKey("agent-a", "ws", "dm", "keep-me", "")
	orphanSessionKey := protocol.BuildAgentSessionKey("agent-a", "ws", "dm", "remove-me", "")
	if err := os.MkdirAll(paths.SessionDir(workspacePath, validSessionKey), 0o755); err != nil {
		t.Fatalf("创建有效 session 目录失败: %v", err)
	}
	if err := os.MkdirAll(paths.SessionDir(workspacePath, orphanSessionKey), 0o755); err != nil {
		t.Fatalf("创建孤儿 session 目录失败: %v", err)
	}

	report, err := migrator.PruneOrphanSessionDirs([]string{workspacePath}, map[string]map[string]struct{}{
		workspacePath: {
			validSessionKey: {},
		},
	})
	if err != nil {
		t.Fatalf("清理孤儿 session 目录失败: %v", err)
	}
	if report.RemovedPaths != 1 {
		t.Fatalf("孤儿 session 目录清理统计不正确: %+v", report)
	}
	if _, err := os.Stat(paths.SessionDir(workspacePath, validSessionKey)); err != nil {
		t.Fatalf("有效 session 目录不应被删除: %v", err)
	}
	if _, err := os.Stat(paths.SessionDir(workspacePath, orphanSessionKey)); !os.IsNotExist(err) {
		t.Fatalf("孤儿 session 目录应已删除: %v", err)
	}
}

func TestHistoryMigratorPrunesOrphanRoomConversationDirs(t *testing.T) {
	workspaceRoot, _ := newHistoryMigratorTestPaths(t)
	paths := New(workspaceRoot)
	migrator := NewHistoryMigrator(workspaceRoot)

	validConversationID := "conversation-keep"
	orphanConversationID := "conversation-remove"
	if err := os.MkdirAll(paths.RoomConversationDir(validConversationID), 0o755); err != nil {
		t.Fatalf("创建有效 room 目录失败: %v", err)
	}
	if err := os.MkdirAll(paths.RoomConversationDir(orphanConversationID), 0o755); err != nil {
		t.Fatalf("创建孤儿 room 目录失败: %v", err)
	}

	report, err := migrator.PruneOrphanRoomConversationDirs(map[string]struct{}{
		validConversationID: {},
	})
	if err != nil {
		t.Fatalf("清理孤儿 room 目录失败: %v", err)
	}
	if report.RemovedPaths != 1 {
		t.Fatalf("孤儿 room 目录清理统计不正确: %+v", report)
	}
	if _, err := os.Stat(paths.RoomConversationDir(validConversationID)); err != nil {
		t.Fatalf("有效 room 目录不应被删除: %v", err)
	}
	if _, err := os.Stat(paths.RoomConversationDir(orphanConversationID)); !os.IsNotExist(err) {
		t.Fatalf("孤儿 room 目录应已删除: %v", err)
	}
}

func newHistoryMigratorTestPaths(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "agent-a")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return workspaceRoot, workspacePath
}

func appendLegacyMessage(
	t *testing.T,
	files *SessionFileStore,
	workspacePath string,
	sessionKey string,
	message sessionmodel.Message,
) {
	t.Helper()
	if err := files.appendJSONL(files.paths.SessionMessagePath(workspacePath, sessionKey), message); err != nil {
		t.Fatalf("写入旧消息失败: %v", err)
	}
}

func writeHistoryTranscriptFixture(
	t *testing.T,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 路径失败: %v", err)
	}
	projectDir := filepath.Join(
		transcriptProjectsDir(),
		sanitizeTranscriptPath(canonicalizeTranscriptPath(workspacePath)),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("创建 transcript fixture 失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 transcript fixture 失败: %v", err)
		}
	}
}
