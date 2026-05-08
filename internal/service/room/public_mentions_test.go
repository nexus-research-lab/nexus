package room

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildPublicMentionSlotKeepsPublicTriggerMessage(t *testing.T) {
	slot := buildPublicMentionSlot(
		&protocol.ConversationContextAggregate{
			Room:         protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
			Conversation: protocol.ConversationRecord{ID: "conversation-1"},
		},
		protocol.SessionRecord{ID: "session-devin"},
		&protocol.Agent{AgentID: "agent-devin", WorkspacePath: t.TempDir()},
		publicMentionWake{
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
			Content:       "@Devin @sam 谁先来？",
			MessageID:     "message-1",
		},
		"round-1",
		"message-slot-1",
		0,
	)

	if slot.Trigger.TriggerType != "public_mention" ||
		slot.Trigger.SourceAgentID != "agent-amy" ||
		slot.Trigger.TargetAgentID != "agent-devin" ||
		slot.Trigger.MessageID != "message-1" ||
		slot.Trigger.Content != "@Devin @sam 谁先来？" {
		t.Fatalf("公区 @ slot 应只保留可直接渲染成消息行的触发信息: %+v", slot.Trigger)
	}
}
