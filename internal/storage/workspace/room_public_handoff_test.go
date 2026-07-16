package workspace

import (
	"testing"
)

func TestRoomPublicHandoffStoreIsDurableAndIdempotent(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-handoff"
	handoff := RoomPublicHandoff{
		HandoffID:          "rh_test-1",
		ConversationID:     conversationID,
		RoomID:             "room-1",
		RootRoundID:        "round-root",
		SourceAgentRoundID: "agent-round-a",
		SourceMessageID:    "message-a",
		SourceAgentID:      "agent-a",
		TargetAgentID:      "agent-b",
		Content:            "请 @AgentB 继续",
	}

	store := NewRoomPublicHandoffStore(root)
	store.paths.HomeRoot = root
	first, inserted, err := store.Detect(handoff)
	if err != nil || !inserted || first.Status != roomPublicHandoffActionDetected {
		t.Fatalf("首次检测应落盘: value=%+v inserted=%v err=%v", first, inserted, err)
	}
	second, inserted, err := store.Detect(handoff)
	if err != nil || inserted || second.HandoffID != handoff.HandoffID {
		t.Fatalf("重复检测必须幂等: value=%+v inserted=%v err=%v", second, inserted, err)
	}
	if err := store.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := store.Claim(conversationID, handoff.HandoffID)
	if err != nil || !ok || claimed.Status != roomPublicHandoffActionClaimed {
		t.Fatalf("source 完成后应只允许一次 claim: value=%+v claimed=%v err=%v", claimed, ok, err)
	}
	if _, ok, err := store.Claim(conversationID, handoff.HandoffID); err != nil || ok {
		t.Fatalf("重复 claim 不应再次成功: ok=%v err=%v", ok, err)
	}
	if err := store.MarkStarted(conversationID, handoff.HandoffID, "target-round"); err != nil {
		t.Fatal(err)
	}

	reloaded := NewRoomPublicHandoffStore(root)
	reloaded.paths.HomeRoot = root
	pending, err := reloaded.Pending(conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("started handoff 不应出现在恢复队列: %+v", pending)
	}
	path := store.paths.RoomPublicHandoffsPath(conversationID)
	if _, err := reloaded.files.readJSONL(path); err != nil {
		t.Fatalf("handoff ledger 应写入 workspace JSONL: %v", err)
	}
}

func TestRoomPublicHandoffStoreReleasesClaimAfterRestartWindow(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-retry"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_test-retry",
		ConversationID:  conversationID,
		SourceMessageID: "message-retry",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
	}
	store := NewRoomPublicHandoffStore(root)
	store.paths.HomeRoot = root
	if _, _, err := store.Detect(handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Claim(conversationID, handoff.HandoffID); err != nil || !ok {
		t.Fatalf("首次 claim 失败: ok=%v err=%v", ok, err)
	}
	// 手工追加一个过期 claim，验证重启扫描会重新暴露它。
	value, ok, err := store.Claim(conversationID, handoff.HandoffID)
	if err != nil || ok {
		t.Fatalf("未过期 claim 不应重复领取: value=%+v ok=%v err=%v", value, ok, err)
	}
	value.ClaimedAt = 1
	value.UpdatedAt = 1
	if err := store.appendLocked(conversationID, roomPublicHandoffActionClaimed, value); err != nil {
		t.Fatal(err)
	}
	reloaded := NewRoomPublicHandoffStore(root)
	reloaded.paths.HomeRoot = root
	pending, err := reloaded.Pending(conversationID)
	if err != nil || len(pending) != 1 || pending[0].HandoffID != handoff.HandoffID {
		t.Fatalf("过期 claim 应进入恢复列表: pending=%+v err=%v", pending, err)
	}
}

func TestRoomPublicHandoffStoreClaimsQueuedDelivery(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-queued"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_test-queued",
		ConversationID:  conversationID,
		SourceMessageID: "message-queued",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
	}
	store := NewRoomPublicHandoffStore(root)
	store.paths.HomeRoot = root
	if _, _, err := store.Detect(handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkQueued(conversationID, handoff.HandoffID, "queue-1"); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := store.Claim(conversationID, handoff.HandoffID)
	if err != nil || !ok || claimed.Status != roomPublicHandoffActionClaimed {
		t.Fatalf("queued handoff 应可被 dispatcher claim: value=%+v ok=%v err=%v", claimed, ok, err)
	}
	if err := store.MarkTerminal(conversationID, handoff.HandoffID, "finished"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending(conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("terminal handoff 不应继续 pending: %+v", pending)
	}
}

func TestRoomPublicHandoffStoreListsAndCancelsRoot(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-root-cancel"
	store := NewRoomPublicHandoffStore(root)
	store.paths.HomeRoot = root
	for _, handoff := range []RoomPublicHandoff{
		{
			HandoffID: "rh-root-a", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-a", SourceAgentID: "agent-a", TargetAgentID: "agent-b",
		},
		{
			HandoffID: "rh-root-b", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-b", SourceAgentID: "agent-b", TargetAgentID: "agent-c",
		},
		{
			HandoffID: "rh-other", ConversationID: conversationID, RootRoundID: "root-2",
			SourceMessageID: "message-c", SourceAgentID: "agent-a", TargetAgentID: "agent-d",
		},
	} {
		if _, _, err := store.Detect(handoff); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
			t.Fatal(err)
		}
	}
	edges, err := store.ListRoot(conversationID, "root-1")
	if err != nil || len(edges) != 2 {
		t.Fatalf("root snapshot 不正确: edges=%+v err=%v", edges, err)
	}
	if err := store.CancelForRoot(conversationID, "root-1", "interrupted"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending(conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].HandoffID != "rh-other" {
		t.Fatalf("取消 root 后只应保留其他 root: %+v", pending)
	}
}
