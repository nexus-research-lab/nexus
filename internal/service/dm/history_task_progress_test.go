package dm

import (
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestAppendRuntimeHistoryMessagePersistsAssistantTaskProgress(t *testing.T) {
	workspaceRoot := t.TempDir()
	workspacePath := filepath.Join(workspaceRoot, "host")
	history := workspacestore.NewAgentHistoryStore(workspaceRoot)
	service := &Service{history: history}
	sessionKey := "agent:host:ws:dm:test"
	session := protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "host",
	}
	message := protocol.Message{
		"message_id":  "assistant-agent-progress",
		"session_key": sessionKey,
		"agent_id":    "host",
		"round_id":    "round-1",
		"role":        "assistant",
		"timestamp":   int64(1000),
		"content": []map[string]any{
			{
				"type":        "task_progress",
				"task_id":     "agent-child-1",
				"tool_use_id": "call-agent",
				"agent_id":    "agent-child-1",
			},
		},
	}

	if err := service.appendRuntimeHistoryMessage(workspacePath, session, message); err != nil {
		t.Fatalf("写入 task progress overlay 失败: %v", err)
	}
	messages, err := history.ReadMessages(workspacePath, session, []string{"round-1"})
	if err != nil {
		t.Fatalf("读取 task progress overlay 失败: %v", err)
	}
	if len(messages) != 1 || messages[0]["message_id"] != "assistant-agent-progress" {
		t.Fatalf("task progress assistant 未进入 durable history: %+v", messages)
	}
}

func TestAppendRuntimeHistoryMessagePersistsAdmittedEchoAssistant(t *testing.T) {
	workspaceRoot := t.TempDir()
	workspacePath := filepath.Join(workspaceRoot, "host")
	history := workspacestore.NewAgentHistoryStore(workspaceRoot)
	service := &Service{history: history}
	sessionKey := "agent:host:ws:dm:echo"
	session := protocol.Session{SessionKey: sessionKey, AgentID: "host"}
	message := protocol.Message{
		"message_id":  "assistant-echo",
		"session_key": sessionKey,
		"agent_id":    "host",
		"round_id":    "round-echo",
		"role":        "assistant",
		"timestamp":   int64(1000),
		"stop_reason": "end_turn",
		"content": []map[string]any{
			{"type": "text", "text": "主动跟进正文"},
		},
		"metadata": map[string]any{"source": "echo"},
	}

	if err := service.appendRuntimeHistoryMessage(workspacePath, session, message); err != nil {
		t.Fatalf("写入 Echo overlay 失败: %v", err)
	}
	messages, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatalf("读取 Echo overlay 失败: %v", err)
	}
	if len(messages) != 1 || messages[0]["message_id"] != "assistant-echo" {
		t.Fatalf("准入后的 Echo assistant 未进入 durable history: %+v", messages)
	}
}
