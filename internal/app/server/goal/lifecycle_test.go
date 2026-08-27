package goal

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
)

type fakeGoalInterruptDM struct {
	requests []dmsvc.InterruptRequest
}

func (f *fakeGoalInterruptDM) HandleInterrupt(_ context.Context, request dmsvc.InterruptRequest) error {
	f.requests = append(f.requests, request)
	return nil
}

type fakeGoalInterruptRoom struct {
	requests []roomrealtime.InterruptRequest
}

func (f *fakeGoalInterruptRoom) HandleInterrupt(_ context.Context, request roomrealtime.InterruptRequest) error {
	f.requests = append(f.requests, request)
	return nil
}

func TestGoalInterruptDispatcherRoutesAgentGoalToDM(t *testing.T) {
	dm := &fakeGoalInterruptDM{}
	room := &fakeGoalInterruptRoom{}
	dispatcher := &goalInterruptDispatcher{dm: dm, room: room}

	if err := dispatcher.InterruptGoalRuntime(context.Background(), "agent:nexus:ws:dm:thread-1", []string{"round-dm"}); err != nil {
		t.Fatalf("InterruptGoalRuntime() error = %v", err)
	}
	if len(dm.requests) != 1 || dm.requests[0].SessionKey != "agent:nexus:ws:dm:thread-1" ||
		dm.requests[0].RoundID != "round-dm" {
		t.Fatalf("dm requests = %#v, want agent session interrupt", dm.requests)
	}
	if len(room.requests) != 0 {
		t.Fatalf("room requests = %#v, want none", room.requests)
	}
}

func TestGoalInterruptDispatcherRoutesRoomGoalToRoom(t *testing.T) {
	dm := &fakeGoalInterruptDM{}
	room := &fakeGoalInterruptRoom{}
	dispatcher := &goalInterruptDispatcher{dm: dm, room: room}

	if err := dispatcher.InterruptGoalRuntime(context.Background(), "room:group:conversation-1", []string{"agent-round-room"}); err != nil {
		t.Fatalf("InterruptGoalRuntime() error = %v", err)
	}
	if len(room.requests) != 1 || room.requests[0].SessionKey != "room:group:conversation-1" ||
		room.requests[0].AgentRoundID != "agent-round-room" {
		t.Fatalf("room requests = %#v, want shared room interrupt", room.requests)
	}
	if len(dm.requests) != 0 {
		t.Fatalf("dm requests = %#v, want none", dm.requests)
	}
}

func TestGoalInterruptDispatcherRoutesRoomAgentGoalToSharedRoom(t *testing.T) {
	room := &fakeGoalInterruptRoom{}
	dispatcher := &goalInterruptDispatcher{room: room}

	if err := dispatcher.InterruptGoalRuntime(context.Background(), "agent:agent-1:ws:group:conversation-1", []string{"agent-round-member"}); err != nil {
		t.Fatalf("InterruptGoalRuntime() error = %v", err)
	}
	if len(room.requests) != 1 || room.requests[0].SessionKey != "room:group:conversation-1" ||
		room.requests[0].AgentRoundID != "agent-round-member" {
		t.Fatalf("room requests = %#v, want shared room session interrupt", room.requests)
	}
}

func TestGoalInterruptDispatcherNeverWidensMissingExactTarget(t *testing.T) {
	dm := &fakeGoalInterruptDM{}
	room := &fakeGoalInterruptRoom{}
	dispatcher := &goalInterruptDispatcher{dm: dm, room: room}

	if err := dispatcher.InterruptGoalRuntime(context.Background(), "room:group:conversation-1", nil); err != nil {
		t.Fatal(err)
	}
	if len(dm.requests) != 0 || len(room.requests) != 0 {
		t.Fatalf("dm=%#v room=%#v, want no session-wide fallback", dm.requests, room.requests)
	}
}

type fakeGoalContinuationDM struct {
	deferResult bool
	missing     bool
	plans       []protocol.GoalContinuation
}

func (f *fakeGoalContinuationDM) ShouldDeferGoalContinuation(context.Context, string, string) bool {
	return f.deferResult
}

func (f *fakeGoalContinuationDM) GoalContinuationTargetMissing(context.Context, string, string) (bool, error) {
	return f.missing, nil
}

func (f *fakeGoalContinuationDM) DispatchGoalContinuation(_ context.Context, plan protocol.GoalContinuation) error {
	f.plans = append(f.plans, plan)
	return nil
}

type fakeGoalContinuationRoom struct {
	deferResult bool
	missing     bool
	checkedRefs []string
	plans       []protocol.GoalContinuation
}

func (f *fakeGoalContinuationRoom) ShouldDeferGoalContinuation(context.Context, string) bool {
	return f.deferResult
}

func (f *fakeGoalContinuationRoom) GoalContinuationTargetMissing(context.Context, string) (bool, error) {
	return f.missing, nil
}

func (f *fakeGoalContinuationRoom) GoalContinuationConversationMissing(_ context.Context, conversationID string) (bool, error) {
	f.checkedRefs = append(f.checkedRefs, conversationID)
	return f.missing, nil
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

func TestGoalContinuationDispatcherDefersUntilTerminalRoundAccountingCleanup(t *testing.T) {
	const sessionKey = "agent:nexus:ws:dm:goal-accounting-cleanup"
	runtime := runtimectx.NewManager()
	runtime.RegisterGoalAccountingCreateGuard(sessionKey, "round-visible", "round-visible", func() bool {
		return true
	})
	runtime.MarkRoundTerminal(sessionKey, "round-visible")
	if got := runtime.GetRunningRoundIDs(sessionKey); len(got) != 0 {
		t.Fatalf("running round ids = %#v, want terminal round absent", got)
	}
	dispatcher := &goalContinuationDispatcher{
		runtime: runtime,
		dm:      &fakeGoalContinuationDM{},
	}
	if !dispatcher.ShouldDeferGoalContinuation(context.Background(), sessionKey) {
		t.Fatal("ShouldDeferGoalContinuation() = false while Goal accounting create guard is live")
	}

	runtime.MarkRoundFinished(sessionKey, "round-visible")
	if dispatcher.ShouldDeferGoalContinuation(context.Background(), sessionKey) {
		t.Fatal("ShouldDeferGoalContinuation() = true after terminal round cleanup")
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
	if len(dm.plans) != 1 || dm.plans[0].RoundID != plan.RoundID || !dm.plans[0].HiddenFromUser {
		t.Fatalf("dm plans = %#v, want hidden continuation plan", dm.plans)
	}
}

func TestGoalContinuationDispatcherChecksAgentGroupConversationTarget(t *testing.T) {
	room := &fakeGoalContinuationRoom{missing: true}
	dm := &fakeGoalContinuationDM{}
	dispatcher := &goalContinuationDispatcher{dm: dm, room: room}

	missing, err := dispatcher.GoalContinuationTargetMissing(context.Background(), "agent:nexus:ws:group:conversation-1")
	if err != nil {
		t.Fatalf("GoalContinuationTargetMissing() error = %v", err)
	}
	if !missing {
		t.Fatal("GoalContinuationTargetMissing() = false, want missing group conversation")
	}
	if len(room.checkedRefs) != 1 || room.checkedRefs[0] != "conversation-1" {
		t.Fatalf("checked refs = %#v, want conversation-1", room.checkedRefs)
	}
}
