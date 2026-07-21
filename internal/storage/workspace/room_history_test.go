package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomHistoryStoreMergesLateMentionAnnotationReference(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	sessionID := "4035f197-ca97-43fc-b9ae-06ac04903213"
	privateSessionKey := "agent:agent-amy:ws:group:conversation-1"
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type": "user", "uuid": "transcript-user-1", "sessionId": sessionID,
			"message": map[string]any{"role": "user", "content": "开始任务"},
		},
		{
			"type": "assistant", "uuid": "transcript-assistant-1", "sessionId": sessionID,
			"parentUuid": "transcript-user-1", "message": map[string]any{
				"id": "assistant-1", "type": "message", "role": "assistant",
				"stop_reason": "end_turn",
				"content":     []map[string]any{{"type": "text", "text": "@Devin 接着处理"}},
			},
		},
	})

	history := NewRoomHistoryStore(workspaceRoot)
	base := protocol.Message{
		"message_id": "assistant-1", "session_id": sessionID,
		"conversation_id": "conversation-1", "agent_id": "agent-amy",
		"round_id": "root-1", "agent_round_id": "agent-round-1",
		"parent_id": "slot-1", "role": "assistant", "is_complete": true,
		"timestamp": int64(1000),
	}
	if err := history.AppendTranscriptReference("conversation-1", workspacePath, privateSessionKey, base); err != nil {
		t.Fatalf("写入初始 transcript 引用失败: %v", err)
	}
	annotated := protocol.Clone(base)
	annotated["agent_mentions"] = []protocol.AgentMention{{
		AgentID: "agent-devin", Label: "Devin", StartRune: 0, EndRune: 6,
	}}
	if err := history.AppendTranscriptReference("conversation-1", workspacePath, privateSessionKey, annotated); err != nil {
		t.Fatalf("写入终态 transcript 引用失败: %v", err)
	}

	rows, err := history.ReadMessages("conversation-1", nil)
	if err != nil {
		t.Fatalf("读取 Room 历史失败: %v", err)
	}
	if len(rows) != 1 || rows[0]["message_id"] != "assistant-1" {
		t.Fatalf("同 message_id 的引用应压缩成一条消息: %+v", rows)
	}
	mentions, ok := rows[0]["agent_mentions"].([]any)
	if !ok || len(mentions) != 1 {
		t.Fatalf("迟到的 agent_mentions 应保留在历史: %+v", rows[0])
	}
}
