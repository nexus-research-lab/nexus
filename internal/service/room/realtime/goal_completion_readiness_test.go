package realtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestActiveRoomGoalBlockerExcludesCallerSlotButKeepsRunningWork(t *testing.T) {
	const conversationID = "conversation-goal-ready"
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	caller := &activeRoomSlot{
		AgentID:      "agent-lead",
		AgentRoundID: "agent-round-lead",
	}
	caller.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round",
		RootRoundID:    "root-round",
		Slots:          map[string]*activeRoomSlot{"caller": caller},
	}
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{"round": roundValue})}

	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); blocker != "" {
		t.Fatalf("caller current slot blocker = %q, want empty", blocker)
	}
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", ""); !strings.Contains(blocker, "active Room slot") {
		t.Fatalf("caller without precise round blocker = %q, want fail-closed active slot", blocker)
	}

	caller.goal.mu.Lock()
	caller.goal.subagentTasks = map[string]struct{}{"task-running": {}}
	caller.goal.mu.Unlock()
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "running subagent work") {
		t.Fatalf("caller subagent blocker = %q, want running subagent work", blocker)
	}
	caller.goal.mu.Lock()
	caller.goal.subagentTasks = nil
	caller.goal.mu.Unlock()

	peer := &activeRoomSlot{AgentID: "agent-peer", AgentRoundID: "agent-round-peer"}
	peer.setStatus("running")
	roundValue.Slots["peer"] = peer
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer") {
		t.Fatalf("peer slot blocker = %q, want active peer", blocker)
	}

	peer.setStatus("finished")
	peer.goal.mu.Lock()
	peer.goal.subagentTasks = map[string]struct{}{"peer-task": {}}
	peer.goal.mu.Unlock()
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer still has running subagent work") {
		t.Fatalf("peer subagent blocker = %q, want peer subagent even after main slot terminal", blocker)
	}
	peer.goal.mu.Lock()
	peer.goal.subagentTasks = nil
	peer.goal.mu.Unlock()

	service.rounds.enqueuePublicMention(roundValue, publicMentionWake{TargetAgentID: "agent-peer"})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "public-mention wake") {
		t.Fatalf("public mention blocker = %q, want pending wake", blocker)
	}
}

func TestRoomGoalInputQueueBlockerClearsOnlyAfterConsumption(t *testing.T) {
	root := t.TempDir()
	store := workspacestore.NewInputQueueStore(root)
	const (
		conversationID = "conversation-goal-queue"
		roomID         = "room-goal-queue"
		agentID        = "agent-peer"
	)
	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  root,
		SessionKey:     protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup),
		RoomID:         roomID,
		ConversationID: conversationID,
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "queued-directed-message",
		AgentID:        agentID,
		SourceAgentID:  "agent-lead",
		Source:         protocol.InputQueueSourceAgentRoomMessage,
		Content:        "continue the delegated comparison",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	}); err != nil {
		t.Fatal(err)
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: roomID, RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members: []protocol.MemberRecord{{
			MemberType: protocol.MemberTypeAgent, MemberAgentID: agentID,
		}},
		MemberAgents: []protocol.Agent{{AgentID: agentID, WorkspacePath: root}},
	}
	service := &Service{inputQueue: store}

	blocker, err := service.roomGoalInputQueueBlocker(context.Background(), contextValue)
	if err != nil || !strings.Contains(blocker, "queued-directed-message") {
		t.Fatalf("queued blocker = %q err=%v, want pending item", blocker, err)
	}
	if _, err = store.Dispatch(location, "queued-directed-message"); err != nil {
		t.Fatal(err)
	}
	blocker, err = service.roomGoalInputQueueBlocker(context.Background(), contextValue)
	if err != nil || blocker != "" {
		t.Fatalf("dispatched blocker = %q err=%v, want empty", blocker, err)
	}
}

func TestRoomGoalDelayedWakeBlockerClearsAfterWakeStarts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := workspacestore.NewRoomDirectedMessageWakeStore(root)
	const conversationID = "conversation-goal-delayed-wake"
	wake := workspacestore.RoomDirectedMessageWake{
		WakeID: "wake-goal",
		Message: protocol.RoomDirectedMessageRecord{
			MessageID:      "wake-goal",
			RoomID:         "room-goal",
			ConversationID: conversationID,
			WakePolicy:     protocol.RoomWakePolicyDelayed,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := store.Schedule(wake); err != nil {
		t.Fatal(err)
	}
	service := &Service{directedWakes: store}

	blocker, err := service.roomGoalDelayedWakeBlocker(conversationID)
	if err != nil || !strings.Contains(blocker, wake.WakeID) {
		t.Fatalf("pending wake blocker = %q err=%v, want wake ID", blocker, err)
	}
	if err = store.Complete(wake.WakeID); err != nil {
		t.Fatal(err)
	}
	blocker, err = service.roomGoalDelayedWakeBlocker(conversationID)
	if err != nil || blocker != "" {
		t.Fatalf("completed wake blocker = %q err=%v, want empty", blocker, err)
	}
}
