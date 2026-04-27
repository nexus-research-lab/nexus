package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		if strings.TrimSpace(stringFromAny(row["round_id"])) != "round-2" {
			continue
		}
		if strings.TrimSpace(stringFromAny(row["role"])) != "assistant" {
			continue
		}
		roundTwoAssistants++
		if got := strings.TrimSpace(stringFromAny(row["message_id"])); got != "transcript-assistant-2" {
			t.Fatalf("第二轮 assistant 不应退化为 synthetic assistant: %+v", row)
		}
		summary, ok := row["result_summary"].(map[string]any)
		if !ok {
			t.Fatalf("第二轮 assistant 应挂载 result_summary: %+v", row)
		}
		if strings.TrimSpace(stringFromAny(summary["subtype"])) != "success" {
			t.Fatalf("第二轮 result_summary subtype 不正确: %+v", summary)
		}
	}

	if roundTwoAssistants != 1 {
		t.Fatalf("第二轮 assistant 数量不正确，说明 result 没有并回同一轮: got=%d rows=%+v", roundTwoAssistants, rows)
	}
}

func TestAgentHistoryStoreSkipsLegacyQueueGuidanceRoundMarkers(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "queue_guide_1", "补充要求", 1000, "guide"); err != nil {
		t.Fatalf("写入旧引导 marker 失败: %v", err)
	}

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "Amy",
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("旧 queue guide marker 不应物化成用户消息: %+v", rows)
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
	if strings.TrimSpace(stringFromAny((*guidance)["content"])) != "需要可以与 bot 对战" {
		t.Fatalf("引导内容解析错误: %+v", *guidance)
	}
	metadata, _ := (*guidance)["metadata"].(map[string]any)
	if metadata["subtype"] != message.SystemMessageSubtypeGuidedInput ||
		metadata["source_round_id"] != "queue_guide_1" {
		t.Fatalf("引导 metadata 不正确: %+v", *guidance)
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
