package workspace

import (
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreRemoveOverlayRounds(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "workspace")
	sessionKey := "agent:nexus:ws:dm:rewrite-prune"
	history := NewAgentHistoryStore(root)

	appendTestRound(t, history, workspacePath, sessionKey, "round-1", "第一问", "第一答", 1000)
	appendTestRound(t, history, workspacePath, sessionKey, "round-2", "旧问题", "旧回答", 2000)
	appendTestRound(t, history, workspacePath, sessionKey, "round-3", "新问题", "新回答", 3100)

	removed, err := history.RemoveOverlayRounds(workspacePath, sessionKey, []string{"round-2"})
	if err != nil {
		t.Fatalf("裁剪 overlay round 失败: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	if hasRound(rows, "round-2") {
		t.Fatalf("被裁剪 round 不应进入有效历史: %+v", rows)
	}
	if !hasRound(rows, "round-1") || !hasRound(rows, "round-3") {
		t.Fatalf("裁剪后应保留其他 round: %+v", rows)
	}

	index, err := history.ReadRoundIndex(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
	}, nil)
	if err != nil {
		t.Fatalf("读取 round index 失败: %v", err)
	}
	if len(index.Items) != 2 {
		t.Fatalf("round index 数量不正确: %+v", index.Items)
	}
	for _, item := range index.Items {
		if item.RoundID == "round-2" {
			t.Fatalf("round index 不应包含被替换 round: %+v", index.Items)
		}
	}
}

func appendTestRound(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	sessionKey string,
	roundID string,
	userContent string,
	assistantContent string,
	timestamp int64,
) {
	t.Helper()
	if err := history.AppendRoundMarker(workspacePath, sessionKey, roundID, userContent, timestamp); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}
	if err := history.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id":  "assistant-" + roundID,
		"session_key": sessionKey,
		"agent_id":    "nexus",
		"round_id":    roundID,
		"role":        "assistant",
		"content": []map[string]any{
			{"type": "text", "text": assistantContent},
		},
		"timestamp": timestamp + 100,
	}); err != nil {
		t.Fatalf("写入 assistant overlay 失败: %v", err)
	}
}

func hasRound(rows []protocol.Message, roundID string) bool {
	for _, row := range rows {
		if stringFromAny(row["round_id"]) == roundID {
			return true
		}
	}
	return false
}
