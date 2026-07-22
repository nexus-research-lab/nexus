package realtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	realRoomCancellationContent = "@Amy 算了不用了"
	realRoomSessionKey          = "room:group:91c68883cc96"
	realGoalID                  = "goal-real-room-review"
)

type cancellationGoalProvider struct {
	current   *protocol.Goal
	clearCall int
	planCall  int
}

func (p *cancellationGoalProvider) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) Clear(_ context.Context, goalID string) (bool, error) {
	p.clearCall++
	if p.current == nil || p.current.ID != goalID {
		return false, nil
	}
	p.current = nil
	return true, nil
}

func (p *cancellationGoalProvider) PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error) {
	p.planCall++
	if p.current == nil {
		return nil, nil
	}
	return &protocol.GoalContinuation{
		Goal:    *p.current,
		RoundID: "goal_continuation_after_cancel",
	}, nil
}

func (p *cancellationGoalProvider) GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error) {
	return p.current != nil, nil
}

func (p *cancellationGoalProvider) ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RuntimeContext(context.Context, string) (string, *protocol.Goal, error) {
	return "", p.current, nil
}

func (p *cancellationGoalProvider) RecordUsageForSession(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordContinuationProgress(context.Context, string, string, bool, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordContinuationFailure(context.Context, string, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordCompletionToolMiss(context.Context, string, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordGoalActivity(context.Context, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordRoomGoalCollaborationRequired(context.Context, string, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordRoomGoalCollaborationEvidence(context.Context, string, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func TestRealRoomCancellationClearsGoalBeforeContinuation(t *testing.T) {
	provider := &cancellationGoalProvider{current: &protocol.Goal{
		ID:         realGoalID,
		SessionKey: realRoomSessionKey,
		Status:     protocol.GoalStatusActive,
	}}
	service := &Service{goals: provider}

	if !isGoalCancellationRequest(realRoomCancellationContent) {
		t.Fatal("真实 Room 引导内容应被识别为明确取消意图")
	}
	if err := service.cancelActiveRoomGoalForUser(
		context.Background(),
		realRoomSessionKey,
		realRoomCancellationContent,
	); err != nil {
		t.Fatalf("清除 active Goal 失败: %v", err)
	}
	if provider.clearCall != 1 || provider.current != nil {
		t.Fatalf("取消应只清除一次 active Goal: calls=%d current=%+v", provider.clearCall, provider.current)
	}

	service.dispatchPostRoundWork(context.Background(), &activeRoomRound{
		SessionKey: realRoomSessionKey,
		RoundID:    "round_after_cancel",
	})
	if provider.planCall != 1 {
		t.Fatalf("取消后应只检查一次续跑且不生成计划: planCall=%d", provider.planCall)
	}
}

func TestGoalCancellationIntentDoesNotMatchOrdinaryDiscussion(t *testing.T) {
	for _, content := range []string{
		"停止后继续执行",
		"请说明任务为什么停止",
		"这个任务已经完成",
	} {
		if isGoalCancellationRequest(content) {
			t.Fatalf("普通讨论不应被识别为取消: %q", content)
		}
	}
}

func TestPublishPublicMessageSuppressesTheSameSlotFinalReply(t *testing.T) {
	slot := &activeRoomSlot{
		AgentID: "agent-amy",
	}
	slot.setPendingStream([]protocol.EventMessage{{EventType: protocol.EventTypeStream}})
	slot.beginNoReplyCandidate()
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
		"round-1": {
			SessionKey:  "room:group:conversation-1",
			RootRoundID: "round-1",
			Slots: map[string]*activeRoomSlot{
				"slot-1": slot,
			},
		},
	})}

	if err := service.MarkPublicMessagePublished(
		context.Background(),
		"room:group:conversation-1",
		"round-1",
		"agent-amy",
	); err != nil {
		t.Fatalf("标记主动广播失败: %v", err)
	}
	if !slot.publicMessageWasPublished() || !slot.shouldSuppressOutput() {
		t.Fatalf("主动广播后 slot 必须进入 suppress 状态: %+v", slot)
	}
	if events := slot.eventsReadyForEmission(protocol.EventMessage{EventType: protocol.EventTypeStream}); len(events) != 0 {
		t.Fatalf("主动广播后不应继续向公区发流事件: %+v", events)
	}
}

// Goal 完成就绪测试。

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

	caller.setSubagentTasks(map[string]struct{}{"task-running": {}})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "running subagent work") {
		t.Fatalf("caller subagent blocker = %q, want running subagent work", blocker)
	}
	caller.setSubagentTasks(nil)

	peer := &activeRoomSlot{AgentID: "agent-peer", AgentRoundID: "agent-round-peer"}
	peer.setStatus("running")
	roundValue.Slots["peer"] = peer
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer") {
		t.Fatalf("peer slot blocker = %q, want active peer", blocker)
	}

	peer.setStatus("finished")
	peer.setSubagentTasks(map[string]struct{}{"peer-task": {}})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer still has running subagent work") {
		t.Fatalf("peer subagent blocker = %q, want peer subagent even after main slot terminal", blocker)
	}
	peer.setSubagentTasks(nil)

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
