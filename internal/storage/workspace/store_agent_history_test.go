package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreMergesOverlayResultIntoTranscriptAssistantAfterEmptyUserTurn(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	sessionID := "093eebdf-c404-428a-964f-68f4a15fe250"

	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-1", "你是谁", 1000); err != nil {
		t.Fatalf("写入第一条 round marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-2", "不是吧", 2000); err != nil {
		t.Fatalf("写入第二条 round marker 失败: %v", err)
	}
	if err := history.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id":      "result-2",
		"session_key":     sessionKey,
		"agent_id":        "Amy",
		"round_id":        "round-2",
		"role":            "result",
		"subtype":         "success",
		"result":          "哈哈，你说得对！看工作区名字，我应该叫 Amy 才对 😊",
		"timestamp":       3000,
		"duration_ms":     5200,
		"duration_api_ms": 4800,
		"num_turns":       1,
		"usage": map[string]any{
			"input_tokens":  99,
			"output_tokens": 94,
		},
	}); err != nil {
		t.Fatalf("写入 overlay result 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-20T19:08:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "你是谁",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "我是当前工作区里的助手。"},
				},
			},
		},
		{
			"type":       "user",
			"uuid":       "transcript-user-empty",
			"sessionId":  sessionID,
			"parentUuid": "transcript-assistant-1",
			"timestamp":  "2026-04-20T19:09:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "",
			},
		},
		{
			"type":       "user",
			"uuid":       "transcript-user-2",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-empty",
			"timestamp":  "2026-04-20T19:09:10.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "不是吧",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-2",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-2",
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "哈哈，你说得对！看工作区名字，我应该叫 Amy 才对 😊"},
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史消息失败: %v", err)
	}

	if len(rows) != 4 {
		t.Fatalf("历史消息数量不正确: got=%d want=4 rows=%+v", len(rows), rows)
	}

	roundTwoAssistants := 0
	for _, row := range rows {
		if stringFromAny(row["round_id"]) != "round-2" {
			continue
		}
		if stringFromAny(row["role"]) != "assistant" {
			continue
		}
		roundTwoAssistants++
		if got := stringFromAny(row["message_id"]); got != "transcript-assistant-2" {
			t.Fatalf("第二轮 assistant 不应退化为 synthetic assistant: %+v", row)
		}
		summary, ok := row["result_summary"].(map[string]any)
		if !ok {
			t.Fatalf("第二轮 assistant 应挂载 result_summary: %+v", row)
		}
		if stringFromAny(summary["subtype"]) != "success" {
			t.Fatalf("第二轮 result_summary subtype 不正确: %+v", summary)
		}
	}

	if roundTwoAssistants != 1 {
		t.Fatalf("第二轮 assistant 数量不正确，说明 result 没有并回同一轮: got=%d rows=%+v", roundTwoAssistants, rows)
	}
}

func TestAgentHistoryStoreProjectsHookAdditionalContextGuidance(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	sessionID := "4035f197-ca97-43fc-b9ae-06ac04903213"
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-1", "写一个五子棋游戏", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-27T11:40:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "写一个五子棋游戏",
			},
		},
		{
			"type":       "attachment",
			"uuid":       "transcript-guidance-1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"timestamp":  "2026-04-27T11:41:00.000Z",
			"attachment": map[string]any{
				"type": "hook_additional_context",
				"content": []string{
					"<nexus_guidance>\n用户在你执行当前 round 时补充了以下引导。请在继续下一步前结合这些要求；如果与原任务冲突，以最新引导为准。\n1. round_id=queue_guide_1: 需要可以与 bot 对战\n</nexus_guidance>",
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}

	var guidance *protocol.Message
	for index := range rows {
		if rows[index]["message_id"] == "queue_guide_1" {
			guidance = &rows[index]
			break
		}
	}
	if guidance == nil {
		t.Fatalf("Claude hook additionalContext 应投影成引导系统消息: %+v", rows)
	}
	if (*guidance)["role"] != "system" || (*guidance)["round_id"] != "round-1" {
		t.Fatalf("引导系统消息应归入当前 round: %+v", *guidance)
	}
	if stringFromAny((*guidance)["content"]) != "需要可以与 bot 对战" {
		t.Fatalf("引导内容解析错误: %+v", *guidance)
	}
	metadata, _ := (*guidance)["metadata"].(map[string]any)
	if metadata["subtype"] != message.SystemMessageSubtypeGuidedInput ||
		metadata["source_round_id"] != "queue_guide_1" {
		t.Fatalf("引导 metadata 不正确: %+v", *guidance)
	}
}

func TestAgentHistoryStoreProjectsWorkspaceFileArtifactFromTranscriptToolResult(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	sessionID := "6a855871-4614-4032-b0b3-62d2358fad99"
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-game", "写一个简单的游戏，html的", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}

	targetPath := filepath.Join(workspacePath, "snake-game.html")
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-game",
			"sessionId": sessionID,
			"timestamp": "2026-05-20T01:07:51.493Z",
			"message": map[string]any{
				"role":    "user",
				"content": "写一个简单的游戏，html的",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-write",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-game",
			"timestamp":  "2026-05-20T01:08:39.805Z",
			"message": map[string]any{
				"id":          "msg_game_write",
				"type":        "message",
				"role":        "assistant",
				"stop_reason": "tool_use",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "好的，我来写一个 HTML 游戏。",
					},
					{
						"type": "tool_use",
						"id":   "tool-write-game",
						"name": "Write",
						"input": map[string]any{
							"file_path": targetPath,
							"content":   "<!doctype html><html><body>game</body></html>",
						},
					},
				},
			},
		},
		{
			"type":       "user",
			"uuid":       "transcript-tool-result",
			"sessionId":  sessionID,
			"parentUuid": "transcript-assistant-write",
			"timestamp":  "2026-05-20T01:08:39.850Z",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "tool-write-game",
						"content":     "File created successfully at: " + targetPath,
					},
				},
			},
			"toolUseResult": map[string]any{
				"type":     "create",
				"filePath": targetPath,
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-final",
			"sessionId":  sessionID,
			"parentUuid": "transcript-tool-result",
			"timestamp":  "2026-05-20T01:08:47.315Z",
			"message": map[string]any{
				"id":          "msg_game_final",
				"type":        "message",
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "游戏写好了，文件在 `snake-game.html`。",
					},
				},
			},
		},
	})

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}

	for _, row := range rows {
		blocks, _ := row["content"].([]map[string]any)
		for _, block := range blocks {
			if block["type"] != protocol.ContentBlockTypeWorkspaceFileArtifact {
				continue
			}
			if block["path"] != "snake-game.html" || block["source_tool_use_id"] != "tool-write-game" {
				t.Fatalf("workspace file artifact 内容不正确: %+v", block)
			}
			return
		}
	}
	t.Fatalf("transcript tool_result 应投影出 workspace_file_artifact: %+v", rows)
}

func TestAgentHistoryStoreRoomPublicCursorIsControlRow(t *testing.T) {
	root := t.TempDir()
	workspacePath := t.TempDir()
	sessionKey := "agent:devin:ws:group:conversation-1"
	store := NewAgentHistoryStore(root)

	if err := store.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id": "visible",
		"role":       "system",
		"content":    "普通 overlay",
		"timestamp":  int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRoomPublicCursor(workspacePath, sessionKey, RoomPublicCursor{
		RoomID:              "room-1",
		ConversationID:      "conversation-1",
		AgentID:             "devin",
		RoundID:             "round-1",
		LastPublicMessageID: "m4",
		LastPublicTimestamp: 4,
		Timestamp:           5,
	}); err != nil {
		t.Fatal(err)
	}

	cursor, ok, err := store.ReadRoomPublicCursor(workspacePath, sessionKey, "conversation-1", "devin")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cursor.LastPublicMessageID != "m4" || cursor.LastPublicTimestamp != 4 {
		t.Fatalf("cursor 读取不正确: ok=%v cursor=%+v", ok, cursor)
	}

	rows, err := store.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "devin",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row["nexus_overlay_kind"] == overlayKindRoomPublicCursor {
			t.Fatalf("公区 cursor 控制行不应进入普通 history: %+v", rows)
		}
	}
}

func writeAgentTranscriptFixture(t *testing.T, workspacePath string, sessionID string, rows []map[string]any) {
	t.Helper()

	projectDir := filepath.Join(
		transcriptProjectsDir(),
		sanitizeTranscriptPath(canonicalizeTranscriptPath(workspacePath)),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 项目目录失败: %v", err)
	}
	transcriptPath := filepath.Join(projectDir, sessionID+".jsonl")

	file, err := os.Create(transcriptPath)
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
