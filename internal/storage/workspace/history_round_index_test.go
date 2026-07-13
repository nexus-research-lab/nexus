package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreReadRoundIndexFromOverlay(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	sessionKey := "agent:amy:ws:dm:user"
	history := NewAgentHistoryStore(root)

	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-1", "第一轮问题", 1000); err != nil {
		t.Fatalf("写入第一轮 marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-2", "第二轮问题", 2000); err != nil {
		t.Fatalf("写入第二轮 marker 失败: %v", err)
	}
	if err := history.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id":  "result_round_1",
		"round_id":    "round-1",
		"role":        "result",
		"subtype":     "success",
		"duration_ms": int64(1500),
		"timestamp":   int64(1800),
	}); err != nil {
		t.Fatalf("写入 result 失败: %v", err)
	}

	index, err := history.ReadRoundIndex(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "amy",
	}, []string{"round-2"})
	if err != nil {
		t.Fatalf("读取 round index 失败: %v", err)
	}
	if len(index.Items) != 2 {
		t.Fatalf("round index 数量不正确: %+v", index)
	}
	if index.Items[0].RoundID != "round-1" || index.Items[0].Title != "第一轮问题" {
		t.Fatalf("第一轮索引不正确: %+v", index.Items[0])
	}
	if index.Items[0].Status != "success" || index.Items[0].DurationMS == nil || *index.Items[0].DurationMS != 1500 {
		t.Fatalf("第一轮 result 元数据不正确: %+v", index.Items[0])
	}
	if !index.Items[0].HasUserMessage || len(index.Items[0].AgentIDs) != 1 || index.Items[0].AgentIDs[0] != "amy" {
		t.Fatalf("第一轮角色索引不正确: %+v", index.Items[0])
	}
	if index.Items[1].RoundID != "round-2" || !index.Items[1].IsLive || index.Items[1].Status != "running" {
		t.Fatalf("第二轮 live 状态不正确: %+v", index.Items[1])
	}
}

func TestAgentHistoryStoreReadRoundIndexIgnoresLargeResultBody(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	sessionKey := "agent:amy:ws:dm:large"
	history := NewAgentHistoryStore(root)

	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-large", "长回复", 1000); err != nil {
		t.Fatalf("写入 marker 失败: %v", err)
	}
	overlayPath := New(root).SessionOverlayPath(workspacePath, sessionKey)
	file, err := os.OpenFile(overlayPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("打开 overlay 失败: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(`{"role":"result","round_id":"round-large","subtype":"success","duration_ms":100,"result":"` + strings.Repeat("x", transcriptScannerBufferBytes+1) + `"}` + "\n"); err != nil {
		t.Fatalf("写入长 result 失败: %v", err)
	}

	index, err := history.ReadRoundIndex(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "amy",
	}, nil)
	if err != nil {
		t.Fatalf("读取长 result round index 失败: %v", err)
	}
	if len(index.Items) != 1 || index.Items[0].RoundID != "round-large" {
		t.Fatalf("长 result 后索引不正确: %+v", index)
	}
	if index.Items[0].Status != "success" || index.Items[0].DurationMS == nil || *index.Items[0].DurationMS != 100 {
		t.Fatalf("长 result 元数据未保留: %+v", index.Items[0])
	}
}

func TestRoomHistoryStoreReadRoundIndexCollapsesAgentRound(t *testing.T) {
	root := t.TempDir()
	history := NewRoomHistoryStore(root)
	conversationID := "conv-1"

	if err := history.AppendInlineMessage(conversationID, protocol.Message{
		"message_id": "user-1",
		"round_id":   "round-room",
		"agent_id":   "amy",
		"role":       "user",
		"content":    "讨论一下实现",
		"timestamp":  int64(1000),
	}); err != nil {
		t.Fatalf("写入 room 用户消息失败: %v", err)
	}
	if err := history.AppendInlineMessage(conversationID, protocol.Message{
		"message_id":  "result-1",
		"round_id":    "round-room:amy",
		"agent_id":    "amy",
		"role":        "result",
		"subtype":     "error",
		"duration_ms": int64(2200),
		"timestamp":   int64(1500),
	}); err != nil {
		t.Fatalf("写入 room result 失败: %v", err)
	}

	index, err := history.ReadRoundIndex(conversationID, nil)
	if err != nil {
		t.Fatalf("读取 room round index 失败: %v", err)
	}
	if len(index.Items) != 1 {
		t.Fatalf("room round index 应折叠为一轮: %+v", index)
	}
	item := index.Items[0]
	if item.RoundID != "round-room" || item.Title != "讨论一下实现" {
		t.Fatalf("room round 基本信息不正确: %+v", item)
	}
	if item.Status != "error" || item.DurationMS == nil || *item.DurationMS != 2200 {
		t.Fatalf("room round result 元数据不正确: %+v", item)
	}
	if !item.HasUserMessage || len(item.AgentIDs) != 1 || item.AgentIDs[0] != "amy" {
		t.Fatalf("room round 角色索引不正确: %+v", item)
	}
}

func TestRoomHistoryStoreReadRoundIndexCollectsMultipleAgents(t *testing.T) {
	root := t.TempDir()
	history := NewRoomHistoryStore(root)
	conversationID := "conv-multi"

	if err := history.AppendInlineMessage(conversationID, protocol.Message{
		"message_id": "user-1",
		"round_id":   "round-room",
		"role":       "user",
		"content":    "多 agent 一起分析",
		"timestamp":  int64(1000),
	}); err != nil {
		t.Fatalf("写入 room 用户消息失败: %v", err)
	}
	for _, agentID := range []string{"amy", "lucy"} {
		if err := history.AppendInlineMessage(conversationID, protocol.Message{
			"message_id": "assistant-" + agentID,
			"round_id":   "round-room:" + agentID,
			"agent_id":   agentID,
			"role":       "assistant",
			"content": []map[string]any{{
				"type": "text",
				"text": agentID + " reply",
			}},
			"timestamp": int64(1200),
		}); err != nil {
			t.Fatalf("写入 room assistant 失败: %v", err)
		}
	}

	index, err := history.ReadRoundIndex(conversationID, nil)
	if err != nil {
		t.Fatalf("读取 room round index 失败: %v", err)
	}
	if len(index.Items) != 1 {
		t.Fatalf("room round index 应折叠为一轮: %+v", index)
	}
	item := index.Items[0]
	if len(item.AgentIDs) != 2 || item.AgentIDs[0] != "amy" || item.AgentIDs[1] != "lucy" {
		t.Fatalf("room 多 agent 索引不正确: %+v", item)
	}
}

func TestRoomHistoryStoreReadRoundIndexUsesLatestMessageRound(t *testing.T) {
	root := t.TempDir()
	history := NewRoomHistoryStore(root)
	conversationID := "conv-guidance"
	message := protocol.Message{
		"message_id": "user-guidance",
		"round_id":   "round-queued",
		"role":       "user",
		"content":    "然后给点建议",
		"timestamp":  int64(1000),
	}
	if err := history.AppendInlineMessage(conversationID, message); err != nil {
		t.Fatalf("写入排队消息失败: %v", err)
	}
	message["round_id"] = "round-goal"
	message["source_round_id"] = "round-queued"
	if err := history.AppendInlineMessage(conversationID, message); err != nil {
		t.Fatalf("写入引导归组消息失败: %v", err)
	}

	index, err := history.ReadRoundIndex(conversationID, nil)
	if err != nil {
		t.Fatalf("读取 room round index 失败: %v", err)
	}
	if len(index.Items) != 1 || index.Items[0].RoundID != "round-goal" {
		t.Fatalf("同一消息应只归入最后写入的 round: %+v", index.Items)
	}
}

func TestRoomHistoryStoreReadRoundIndexCollapsesSuffixedMarker(t *testing.T) {
	root := t.TempDir()
	history := NewRoomHistoryStore(root)
	conversationID := "conv-suffixed-marker"

	if err := history.AppendInlineMessage(conversationID, protocol.Message{
		overlayKindField: overlayKindRoundMarker,
		"round_id":       "room_mention_abc:amy",
		"content":        "继续推进这个问题",
		"timestamp":      int64(1000),
	}); err != nil {
		t.Fatalf("写入 room marker 失败: %v", err)
	}
	if err := history.AppendInlineMessage(conversationID, protocol.Message{
		"message_id":  "result-amy",
		"round_id":    "room_mention_abc:amy",
		"agent_id":    "amy",
		"role":        "result",
		"subtype":     "success",
		"duration_ms": int64(1800),
		"timestamp":   int64(2000),
	}); err != nil {
		t.Fatalf("写入 room result 失败: %v", err)
	}

	index, err := history.ReadRoundIndex(conversationID, nil)
	if err != nil {
		t.Fatalf("读取 room round index 失败: %v", err)
	}
	if len(index.Items) != 1 {
		t.Fatalf("带 agent 后缀的 marker/result 应折叠为一轮: %+v", index)
	}
	item := index.Items[0]
	if item.RoundID != "room_mention_abc" || item.Title != "继续推进这个问题" {
		t.Fatalf("带 agent 后缀的 marker 归一不正确: %+v", item)
	}
	if !item.HasUserMessage || len(item.AgentIDs) != 1 || item.AgentIDs[0] != "amy" {
		t.Fatalf("带 agent 后缀的 marker 角色索引不正确: %+v", item)
	}
}
