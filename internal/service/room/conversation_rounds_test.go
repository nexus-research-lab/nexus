package room

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomRoundRegistryKeepsPublicWakeAfterRoundUnregister(t *testing.T) {
	const conversationID = "conversation-public-wake-lifecycle"
	roundValue := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		ConversationID: conversationID,
		RoundID:        "round-public-wake",
		Slots:          make(map[string]*activeRoomSlot),
	}
	registry := newRoomRoundRegistry()
	registry.register(roundValue)
	wake := publicMentionWake{TargetAgentID: "agent-peer", Content: "继续处理"}
	if !registry.enqueuePublicMention(roundValue, wake) {
		t.Fatal("首次 public wake 入队失败")
	}

	registry.unregister(roundValue)
	if !registry.hasPublicMentions(roundValue) {
		t.Fatal("round 注销后不应丢失待处理 public wake")
	}
	if !registry.hasPublicMentionsForConversation(conversationID) {
		t.Fatal("round 注销后 conversation 仍应报告待处理 public wake")
	}
	wakes := registry.takePublicMentions(roundValue)
	if len(wakes) != 1 || wakes[0].TargetAgentID != wake.TargetAgentID {
		t.Fatalf("取出的 public wake = %+v, want %+v", wakes, wake)
	}
	if registry.hasPublicMentions(roundValue) {
		t.Fatal("public wake 消费后仍残留")
	}
	if registry.hasPublicMentionsForConversation(conversationID) {
		t.Fatal("public wake 消费后 conversation 仍报告 pending wake")
	}
	if got := len(registry.snapshotConversation(conversationID)); got != 0 {
		t.Fatalf("round 注销后 active round 数 = %d, want 0", got)
	}
}
