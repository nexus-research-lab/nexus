package realtime

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomChatExecutionBuildRoundSeparatesSlashTriggerFromAtomicRuntimeInput(t *testing.T) {
	const rawSlash = "/visualize quarterly revenue"
	const runtimePrompt = "Use Generative UI to create an interactive visual"
	execution := roomChatExecution{
		ctx: context.Background(),
		request: ChatRequest{
			Content:       rawSlash,
			RoundID:       "round-visualize",
			UserMessageID: "message-visualize",
		},
		runtimeTriggerText: rawSlash,
		atomicRuntimeInput: runtimePrompt,
		contextValue: &protocol.ConversationContextAggregate{
			Room: protocol.RoomRecord{RoomType: protocol.RoomTypeGroup},
			Sessions: []protocol.SessionRecord{{
				ID:      "session-visualize",
				AgentID: "agent-visualize",
			}},
		},
		agentByID: map[string]*protocol.Agent{
			"agent-visualize": {
				AgentID:       "agent-visualize",
				WorkspacePath: "agent-visualize",
			},
		},
		targetAgentIDs: []string{"agent-visualize"},
		userMessage:    protocol.Message{"timestamp": int64(1)},
	}

	roundValue, _ := execution.buildRound()
	for _, slot := range roundValue.Slots {
		if slot.Trigger.Content != rawSlash {
			t.Fatalf("Room trigger = %q, want raw Slash %q", slot.Trigger.Content, rawSlash)
		}
		if slot.AtomicRuntimeInput != runtimePrompt {
			t.Fatalf("Room atomic runtime input = %q, want %q", slot.AtomicRuntimeInput, runtimePrompt)
		}
		return
	}
	t.Fatal("Room round has no slot")
}

func TestGetActiveRoundSnapshotKeepsPerSlotRootAcrossConcurrentRounds(
	t *testing.T,
) {
	const conversationID = "conversation-multi-root-snapshot"
	firstSlot := &activeRoomSlot{
		AgentID:      "agent-a",
		AgentRoundID: "agent-round-a",
		MsgID:        "slot-a",
		TimestampMS:  10,
	}
	firstSlot.setStatus("running")
	firstSlot.setDeliveryMetadata(
		protocol.RoomReplyRoute{},
		"source-message-a",
		"handoff-a",
	)
	secondSlot := &activeRoomSlot{
		AgentID:      "agent-b",
		AgentRoundID: "agent-round-b",
		MsgID:        "slot-b",
		TimestampMS:  20,
	}
	secondSlot.setStatus("pending")

	service := &Service{
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"runtime-round-a": {
				SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
				RoomID:         "room-multi-root-snapshot",
				ConversationID: conversationID,
				RoundID:        "runtime-round-a",
				RootRoundID:    "root-a",
				Slots:          map[string]*activeRoomSlot{firstSlot.MsgID: firstSlot},
			},
			"runtime-round-b": {
				SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
				RoomID:         "room-multi-root-snapshot",
				ConversationID: conversationID,
				RoundID:        "runtime-round-b",
				RootRoundID:    "root-b",
				Slots:          map[string]*activeRoomSlot{secondSlot.MsgID: secondSlot},
			},
		}),
	}

	snapshot := service.GetActiveRoundSnapshot(conversationID)
	if snapshot == nil {
		t.Fatal("GetActiveRoundSnapshot() = nil")
	}
	if snapshot.RoundID != "" {
		t.Fatalf("aggregate RoundID = %q, want empty for multiple roots", snapshot.RoundID)
	}
	rootsByMessageID := make(map[string]string, len(snapshot.Pending))
	handoffsByMessageID := make(map[string]string, len(snapshot.Pending))
	for _, slot := range snapshot.Pending {
		rootsByMessageID[slot.MsgID] = slot.RoundID
		handoffsByMessageID[slot.MsgID] = slot.HandoffID
	}
	if rootsByMessageID[firstSlot.MsgID] != "root-a" {
		t.Fatalf("slot-a root = %q, want root-a", rootsByMessageID[firstSlot.MsgID])
	}
	if rootsByMessageID[secondSlot.MsgID] != "root-b" {
		t.Fatalf("slot-b root = %q, want root-b", rootsByMessageID[secondSlot.MsgID])
	}
	if handoffsByMessageID[firstSlot.MsgID] != "handoff-a" {
		t.Fatalf(
			"slot-a handoff = %q, want handoff-a",
			handoffsByMessageID[firstSlot.MsgID],
		)
	}
	if handoffsByMessageID[secondSlot.MsgID] != "" {
		t.Fatalf(
			"ordinary slot handoff = %q, want empty",
			handoffsByMessageID[secondSlot.MsgID],
		)
	}
}
