package room

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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
	service := &RealtimeService{goals: provider}

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
		AgentID:          "agent-amy",
		PendingStream:    []protocol.EventMessage{{EventType: protocol.EventTypeStream}},
		NoReplyCandidate: true,
	}
	service := &RealtimeService{activeRounds: map[string]*activeRoomRound{
		"round-1": {
			SessionKey:  "room:group:conversation-1",
			RootRoundID: "round-1",
			Slots: map[string]*activeRoomSlot{
				"slot-1": slot,
			},
		},
	}}

	if err := service.MarkPublicMessagePublished(
		context.Background(),
		"room:group:conversation-1",
		"round-1",
		"agent-amy",
	); err != nil {
		t.Fatalf("标记主动广播失败: %v", err)
	}
	if !slot.PublicMessagePublished || !slot.shouldSuppressOutput() {
		t.Fatalf("主动广播后 slot 必须进入 suppress 状态: %+v", slot)
	}
	if events := slot.eventsReadyForEmission(protocol.EventMessage{EventType: protocol.EventTypeStream}); len(events) != 0 {
		t.Fatalf("主动广播后不应继续向公区发流事件: %+v", events)
	}
}
