package workspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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

func TestAgentHistoryStoreResolveTranscriptRoundTail(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

	sessionKey := "agent:nexus:ws:dm:rewrite-tail"
	sessionID := "session-rewrite-tail"
	history := NewAgentHistoryStore(workspaceRoot)
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-1", "第一问", 1000); err != nil {
		t.Fatalf("写入 round-1 marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-2", "旧问题", 2000); err != nil {
		t.Fatalf("写入 round-2 marker 失败: %v", err)
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "user-1",
			"sessionId": sessionID,
			"timestamp": "2026-07-08T01:00:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "第一问",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-1",
			"sessionId":  sessionID,
			"parentUuid": "user-1",
			"timestamp":  "2026-07-08T01:00:01.000Z",
			"message": map[string]any{
				"id":      "assistant-1",
				"type":    "message",
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "第一答"}},
			},
		},
		{
			"type":       "user",
			"uuid":       "user-2",
			"sessionId":  sessionID,
			"parentUuid": "assistant-1",
			"timestamp":  "2026-07-08T01:00:02.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "旧问题",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-2",
			"sessionId":  sessionID,
			"parentUuid": "user-2",
			"timestamp":  "2026-07-08T01:00:03.000Z",
			"message": map[string]any{
				"id":      "assistant-2",
				"type":    "message",
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "旧回答"}},
			},
		},
	})

	tail, err := history.ResolveTranscriptRoundTail(workspacePath, sessionKey, sessionID, "round-2")
	if err != nil {
		t.Fatalf("解析 rewrite tail 失败: %v", err)
	}
	if tail.TargetMessageUUID != "user-2" {
		t.Fatalf("target uuid = %q, want user-2", tail.TargetMessageUUID)
	}
	if tail.TargetRoundEndUUID != "assistant-2" {
		t.Fatalf("target end uuid = %q, want assistant-2", tail.TargetRoundEndUUID)
	}
	if want := []string{"user-2", "assistant-2"}; !reflect.DeepEqual(tail.MessageUUIDs, want) {
		t.Fatalf("tail uuids = %#v, want %#v", tail.MessageUUIDs, want)
	}
	if want := []string{"round-2"}; !reflect.DeepEqual(tail.RoundIDs, want) {
		t.Fatalf("tail round ids = %#v, want %#v", tail.RoundIDs, want)
	}
	firstRound, err := history.ResolveTranscriptRoundTail(
		workspacePath,
		sessionKey,
		sessionID,
		"round-1",
	)
	if err != nil {
		t.Fatalf("解析历史 round 边界失败: %v", err)
	}
	if firstRound.TargetRoundEndUUID != "assistant-1" {
		t.Fatalf("historical round end uuid = %q, want assistant-1", firstRound.TargetRoundEndUUID)
	}
}

func TestAgentHistoryStoreForkRoundMarkers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "home"))
	workspacePath := filepath.Join(root, "workspace")
	sourceSessionKey := "agent:nexus:ws:dm:fork-source"
	targetSessionKey := "agent:nexus:ws:dm:fork-target"
	history := NewAgentHistoryStore(root)

	for index, roundID := range []string{"round-1", "round-2", "round-3"} {
		if err := history.AppendRoundMarkerWithOptions(
			workspacePath,
			sourceSessionKey,
			roundID,
			"问题 "+roundID,
			int64(index+1)*1000,
			RoundMarkerOptions{ClientMessageID: "client-" + roundID},
		); err != nil {
			t.Fatalf("写入 %s marker 失败: %v", roundID, err)
		}
	}

	if err := history.ForkRoundMarkers(
		workspacePath,
		sourceSessionKey,
		targetSessionKey,
		"round-2",
	); err != nil {
		t.Fatalf("复制 fork marker 失败: %v", err)
	}
	target, err := history.readOverlayHistoryState(workspacePath, targetSessionKey)
	if err != nil {
		t.Fatalf("读取 fork marker 失败: %v", err)
	}
	if len(target.RoundMarkers) != 2 {
		t.Fatalf("fork marker 数量 = %d, want 2", len(target.RoundMarkers))
	}
	if target.RoundMarkers[1].RoundID != "round-2" ||
		target.RoundMarkers[1].ClientMessageID != "client-round-2" {
		t.Fatalf("fork marker 元数据不正确: %+v", target.RoundMarkers[1])
	}

	sessionID := "fork-source-session"
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type": "user", "uuid": "user-1", "sessionId": sessionID,
			"timestamp": 1000, "message": map[string]any{"role": "user", "content": "问题 round-1"},
		},
		{
			"type": "assistant", "uuid": "assistant-1", "parentUuid": "user-1", "sessionId": sessionID,
			"timestamp": 1100, "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "回答 round-1"}}},
		},
		{
			"type": "user", "uuid": "user-2", "parentUuid": "assistant-1", "sessionId": sessionID,
			"timestamp": 2000, "message": map[string]any{"role": "user", "content": "问题 round-2"},
		},
		{
			"type": "assistant", "uuid": "assistant-2", "parentUuid": "user-2", "sessionId": sessionID,
			"timestamp": 2100, "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "回答 round-2"}}},
		},
		{
			"type": "user", "uuid": "user-3", "parentUuid": "assistant-2", "sessionId": sessionID,
			"timestamp": 3000, "message": map[string]any{"role": "user", "content": "问题 round-3"},
		},
		{
			"type": "assistant", "uuid": "assistant-3", "parentUuid": "user-3", "sessionId": sessionID,
			"timestamp": 3100, "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "回答 round-3"}}},
		},
	})
	pendingFork := protocol.Session{
		SessionKey: targetSessionKey,
		AgentID:    "nexus",
		Options: map[string]any{
			protocol.OptionRuntimeForkSourceSessionID: sessionID,
			protocol.OptionRuntimeForkMessageID:       "assistant-2",
		},
	}
	rows, err := history.ReadMessages(workspacePath, pendingFork, nil)
	if err != nil {
		t.Fatalf("读取 pending fork 历史失败: %v", err)
	}
	if !hasRound(rows, "round-1") || !hasRound(rows, "round-2") || hasRound(rows, "round-3") {
		t.Fatalf("pending fork 历史未停在目标轮次: %+v", rows)
	}
	page, err := history.ReadMessagesPageContext(
		context.Background(),
		workspacePath,
		pendingFork,
		nil,
		HistoryPageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatalf("分页读取 pending fork 历史失败: %v", err)
	}
	if !hasRound(page.Items, "round-1") || !hasRound(page.Items, "round-2") || hasRound(page.Items, "round-3") {
		t.Fatalf("分页 pending fork 历史越过目标轮次: %+v", page.Items)
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
