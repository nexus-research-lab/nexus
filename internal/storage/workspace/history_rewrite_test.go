package workspace

import (
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreAppliesHistoryRewrite(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "workspace")
	sessionKey := "agent:nexus:ws:dm:rewrite"
	history := NewAgentHistoryStore(root)

	appendTestRound(t, history, workspacePath, sessionKey, "round-1", "第一问", "第一答", 1000)
	appendTestRound(t, history, workspacePath, sessionKey, "round-2", "旧问题", "旧回答", 2000)
	if err := history.AppendHistoryRewrite(workspacePath, sessionKey, HistoryRewriteOptions{
		TargetRoundID:      "round-2",
		ReplacementRoundID: "round-3",
		Content:            "新问题",
		Timestamp:          3000,
	}); err != nil {
		t.Fatalf("写入 rewrite marker 失败: %v", err)
	}
	appendTestRound(t, history, workspacePath, sessionKey, "round-3", "新问题", "新回答", 3100)

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}

	if hasRound(rows, "round-2") {
		t.Fatalf("被替换 round 不应进入有效历史: %+v", rows)
	}
	if !hasRound(rows, "round-1") || !hasRound(rows, "round-3") {
		t.Fatalf("有效历史应保留替换前历史和 replacement round: %+v", rows)
	}
}

func TestAgentHistoryStoreAppliesRepeatedHistoryRewrite(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "workspace")
	sessionKey := "agent:nexus:ws:dm:rewrite-repeat"
	history := NewAgentHistoryStore(root)

	appendTestRound(t, history, workspacePath, sessionKey, "round-1", "第一问", "第一答", 1000)
	appendTestRound(t, history, workspacePath, sessionKey, "round-2", "旧问题", "旧回答", 2000)
	if err := history.AppendHistoryRewrite(workspacePath, sessionKey, HistoryRewriteOptions{
		TargetRoundID:      "round-2",
		ReplacementRoundID: "round-3",
		Content:            "第二版问题",
		Timestamp:          3000,
	}); err != nil {
		t.Fatalf("写入第一次 rewrite marker 失败: %v", err)
	}
	appendTestRound(t, history, workspacePath, sessionKey, "round-3", "第二版问题", "第二版回答", 3100)
	if err := history.AppendHistoryRewrite(workspacePath, sessionKey, HistoryRewriteOptions{
		TargetRoundID:      "round-3",
		ReplacementRoundID: "round-4",
		Content:            "第三版问题",
		Timestamp:          4000,
	}); err != nil {
		t.Fatalf("写入第二次 rewrite marker 失败: %v", err)
	}
	appendTestRound(t, history, workspacePath, sessionKey, "round-4", "第三版问题", "第三版回答", 4100)

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}

	if hasRound(rows, "round-2") || hasRound(rows, "round-3") {
		t.Fatalf("连续 rewrite 应只保留最后 replacement round: %+v", rows)
	}
	if !hasRound(rows, "round-1") || !hasRound(rows, "round-4") {
		t.Fatalf("连续 rewrite 后有效历史不正确: %+v", rows)
	}
}

func TestAgentHistoryStoreReadRoundIndexSkipsRewrittenRound(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "workspace")
	sessionKey := "agent:nexus:ws:dm:rewrite-index"
	history := NewAgentHistoryStore(root)

	appendTestRound(t, history, workspacePath, sessionKey, "round-1", "第一问", "第一答", 1000)
	appendTestRound(t, history, workspacePath, sessionKey, "round-2", "旧问题", "旧回答", 2000)
	if err := history.AppendHistoryRewrite(workspacePath, sessionKey, HistoryRewriteOptions{
		TargetRoundID:      "round-2",
		ReplacementRoundID: "round-3",
		Content:            "新问题",
		Timestamp:          3000,
	}); err != nil {
		t.Fatalf("写入 rewrite marker 失败: %v", err)
	}
	appendTestRound(t, history, workspacePath, sessionKey, "round-3", "新问题", "新回答", 3100)

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
