package realtime

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestRoomGoalCompletionReceiptPersistsAndSilentlyEnrichesPublicReply(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	const (
		ownerUserID  = "owner-goal-receipt"
		conversation = "conversation-goal-receipt"
		goalID       = "goal-room-1"
		agentID      = "agent-room-1"
		agentRoundID = "agent-round-room-1"
	)
	contextValue := newAuthorityFenceContext()
	contextValue.Room.ID = "room-goal-receipt"
	contextValue.Room.OwnerUserID = ownerUserID
	contextValue.Conversation.ID = conversation
	contextValue.Conversation.RoomID = contextValue.Room.ID
	contextValue.Members[0].MemberAgentID = agentID
	store := &authorityFenceRoomStore{contextValue: contextValue}
	roundValue := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversation),
		RoomID:         contextValue.Room.ID,
		ConversationID: conversation,
		Context:        cloneAuthorityFenceContext(contextValue),
		AuthorityEpoch: contextValue.Room.AuthorityEpoch,
		RootRoundID:    "root-round-room-1",
		RoundID:        "root-round-room-1",
		OwnerUserID:    ownerUserID,
	}
	workspacePath := filepath.Join(appfs.UserWorkspaceRootAt(root, ownerUserID), agentID)
	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(conversation, agentID, protocol.RoomTypeGroup)
	assistant := protocol.Message{
		"message_id":     "assistant-room-final",
		"session_key":    roundValue.SessionKey,
		"agent_id":       agentID,
		"round_id":       roundValue.RootRoundID,
		"agent_round_id": agentRoundID,
		"role":           "assistant",
		"timestamp":      int64(1000),
		"content":        []map[string]any{{"type": "text", "text": "Room 最终交付"}},
	}
	slot := &activeRoomSlot{
		OwnerUserID:       ownerUserID,
		AgentID:           agentID,
		AgentRoundID:      agentRoundID,
		RuntimeSessionKey: runtimeSessionKey,
		WorkspacePath:     workspacePath,
	}
	slot.markGoalCompletionCandidate(goalID)
	slot.rememberGoalCompletionAssistant(assistant)
	roundValue.Slots = map[string]*activeRoomSlot{"lead": slot}
	provider := &fakeRoomGoalUsageFinalizer{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:          goalID,
			SessionKey:      roundValue.SessionKey,
			Status:          protocol.GoalStatusComplete,
			TimeUsedSeconds: 754,
		},
	}
	service := &Service{
		rooms:       store,
		goals:       provider,
		history:     workspacestore.NewAgentHistoryStore(root),
		roomHistory: workspacestore.NewRoomHistoryStore(root),
		broadcaster: &authorityFenceBroadcaster{},
	}

	service.persistRoomGoalCompletionReceipt(context.Background(), roundValue, slot, false)
	first := readRoomGoalCompletionReceipt(t, root, roundValue, slot)
	if first.TimeUsedSeconds == nil || *first.TimeUsedSeconds != 754 || first.ActualTokens != nil {
		t.Fatalf("first receipt = %+v, want duration only", first)
	}

	provider.mu.Lock()
	provider.report.UsageFinalized = true
	provider.report.Usage = protocol.GoalUsage{ActualTotalTokens: 62762, ActualTotalKnown: true}
	provider.mu.Unlock()
	service.persistRoomGoalCompletionReceipt(context.Background(), roundValue, slot, true)
	final := readRoomGoalCompletionReceipt(t, root, roundValue, slot)
	if final.ActualTokens == nil || *final.ActualTokens != 62762 {
		t.Fatalf("final receipt = %+v, want authoritative actual tokens", final)
	}

	shared, err := service.roomHistory.ReadMessages(ownerUserID, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	if countRoomMessagesByID(shared, "assistant-room-final") != 1 {
		t.Fatalf("shared messages = %+v, want one merged final assistant", shared)
	}
}

func readRoomGoalCompletionReceipt(
	t *testing.T,
	root string,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) protocol.GoalCompletionReceipt {
	t.Helper()
	messages, err := workspacestore.NewAgentHistoryStore(root).
		ForOwner(roundValue.OwnerUserID).
		ReadMessages(slot.WorkspacePath, protocol.Session{
			SessionKey: slot.RuntimeSessionKey,
			AgentID:    slot.AgentID,
			Options:    map[string]any{},
		}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if message["message_id"] != "assistant-room-final" {
			continue
		}
		raw, ok := message[protocol.GoalCompletionReceiptField].(map[string]any)
		if !ok {
			t.Fatalf("receipt missing from private history: %+v", message)
		}
		receipt := protocol.GoalCompletionReceipt{
			GoalID:  anyString(raw["goal_id"]),
			RoundID: anyString(raw["round_id"]),
		}
		if value, ok := roomReceiptInt64(raw["time_used_seconds"]); ok {
			receipt.TimeUsedSeconds = &value
		}
		if value, ok := roomReceiptInt64(raw["actual_tokens"]); ok {
			receipt.ActualTokens = &value
		}
		return receipt
	}
	t.Fatalf("final assistant missing: %+v", messages)
	return protocol.GoalCompletionReceipt{}
}

func countRoomMessagesByID(messages []protocol.Message, messageID string) int {
	count := 0
	for _, message := range messages {
		if anyString(message["message_id"]) == messageID {
			count++
		}
	}
	return count
}

func roomReceiptInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}
