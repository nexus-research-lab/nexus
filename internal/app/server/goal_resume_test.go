package server

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
)

type fakeGoalContinuationDM struct {
	deferResult bool
	requests    []dmsvc.Request
}

func (f *fakeGoalContinuationDM) ShouldDeferGoalContinuation(context.Context, string, string) bool {
	return f.deferResult
}

func (f *fakeGoalContinuationDM) HandleChat(_ context.Context, request dmsvc.Request) error {
	f.requests = append(f.requests, request)
	return nil
}

type fakeGoalContinuationRoom struct {
	deferResult bool
	plans       []protocol.GoalContinuation
}

func (f *fakeGoalContinuationRoom) ShouldDeferGoalContinuation(context.Context, string) bool {
	return f.deferResult
}

func (f *fakeGoalContinuationRoom) DispatchGoalContinuation(_ context.Context, plan protocol.GoalContinuation) error {
	f.plans = append(f.plans, plan)
	return nil
}

func TestGoalContinuationDispatcherDispatchesRoomGoal(t *testing.T) {
	room := &fakeGoalContinuationRoom{}
	dispatcher := &goalContinuationDispatcher{room: room}
	plan := protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID:         "goal-room",
			SessionKey: "room:group:conversation-1",
			Status:     protocol.GoalStatusActive,
		},
		RoundID:        "goal_continuation_1",
		Prompt:         "Continue the shared room goal.",
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
		Metadata:       map[string]string{"goal_id": "goal-room"},
	}

	if err := dispatcher.DispatchGoalContinuation(context.Background(), plan); err != nil {
		t.Fatalf("DispatchGoalContinuation() error = %v", err)
	}
	if len(room.plans) != 1 || room.plans[0].RoundID != plan.RoundID {
		t.Fatalf("room plans = %#v, want dispatched room continuation", room.plans)
	}
}

func TestGoalContinuationDispatcherAsksRoomBeforeAutoContinuing(t *testing.T) {
	room := &fakeGoalContinuationRoom{deferResult: true}
	dispatcher := &goalContinuationDispatcher{room: room}

	if !dispatcher.ShouldDeferGoalContinuation(context.Background(), "room:group:conversation-1") {
		t.Fatal("ShouldDeferGoalContinuation() = false, want room defer result")
	}
}

func TestGoalContinuationDispatcherKeepsAgentDispatch(t *testing.T) {
	dm := &fakeGoalContinuationDM{}
	dispatcher := &goalContinuationDispatcher{dm: dm}
	plan := protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID:         "goal-agent",
			SessionKey: "agent:nexus:ws:dm:thread-1",
			Status:     protocol.GoalStatusActive,
		},
		RoundID:        "goal_continuation_1",
		Prompt:         "Continue the DM goal.",
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
		Metadata:       map[string]string{"goal_id": "goal-agent"},
	}

	if err := dispatcher.DispatchGoalContinuation(context.Background(), plan); err != nil {
		t.Fatalf("DispatchGoalContinuation() error = %v", err)
	}
	if len(dm.requests) != 1 || !dm.requests[0].Internal || !dm.requests[0].InputOptions.HiddenFromUser {
		t.Fatalf("dm requests = %#v, want hidden internal continuation", dm.requests)
	}
}
