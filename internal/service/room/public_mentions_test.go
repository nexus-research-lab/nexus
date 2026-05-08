package room

import (
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildPublicMentionSlotAddsFanoutMetadata(t *testing.T) {
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
		[]string{"agent-devin", "agent-sam"},
		map[string]string{
			"agent-devin": "Devin",
			"agent-sam":   "sam",
		},
	)

	metadata := slot.Trigger.Metadata
	if metadata["public_mention_target_count"] != 2 {
		t.Fatalf("公区 @ 元数据缺少目标数量: %+v", metadata)
	}
	if metadata["public_mention_target_index"] != 0 {
		t.Fatalf("公区 @ 元数据缺少目标顺序: %+v", metadata)
	}
	names, ok := metadata["public_mention_target_names"].([]string)
	if !ok || !slices.Equal(names, []string{"Devin", "sam"}) {
		t.Fatalf("公区 @ 元数据目标名称不正确: %+v", metadata)
	}
}
