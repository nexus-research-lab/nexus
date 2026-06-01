package room

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoomRoundInputOptionsMarksInternalContinuationHidden(t *testing.T) {
	roundValue := &activeRoomRound{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:  "goal_continuation",
			Metadata: map[string]string{"goal_id": "goal-room"},
		},
	}

	options := roomRoundInputOptions(roundValue)

	if !options.HiddenFromUser || !options.Synthetic || options.Priority != "internal" {
		t.Fatalf("options = %#v, want hidden synthetic internal continuation", options)
	}
	if options.Purpose != "goal_continuation" || options.Metadata["goal_id"] != "goal-room" {
		t.Fatalf("options = %#v, want continuation metadata preserved", options)
	}
}

func TestRoomRuntimeVisibleInputOptionsClearsGoalContinuationRuntimeFlags(t *testing.T) {
	options := runtimectx.VisibleInputOptionsForPurpose(sdkprotocol.OutboundMessageOptions{
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
		Priority:       "internal",
		Metadata:       map[string]string{"goal_id": "goal-room"},
	}, "goal_continuation")

	if options.HiddenFromUser || options.Synthetic || options.Purpose != "" || options.Priority != "" || len(options.Metadata) > 0 {
		t.Fatalf("runtime options = %#v, want visible normal runtime input", options)
	}
}

func TestRoomRoundMarkerOptionsMarksInternalContinuationHidden(t *testing.T) {
	roundValue := &activeRoomRound{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:  "goal_continuation",
			Metadata: map[string]string{"goal_id": "goal-room"},
		},
	}

	options := roomRoundMarkerOptions(roundValue)

	if !options.HiddenFromUser || !options.Synthetic {
		t.Fatalf("options = %#v, want hidden synthetic round marker", options)
	}
	if options.Purpose != "goal_continuation" || options.Metadata["goal_id"] != "goal-room" {
		t.Fatalf("options = %#v, want continuation metadata preserved", options)
	}
}

func TestInitialRoomTriggerTypeUsesGoalContinuationForInternalContinuation(t *testing.T) {
	triggerType := initialRoomTriggerType(ChatRequest{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}, "room_host_default")

	if triggerType != "goal_continuation" {
		t.Fatalf("triggerType = %q, want goal_continuation", triggerType)
	}
}

func TestRealtimeServicePostRoundWorkPlansRoomGoalContinuation(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{
		goals: goalProvider,
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
	}

	service.dispatchPostRoundWork(context.Background(), roundValue)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if goalProvider.planCalls != 1 {
		t.Fatalf("planCalls = %d, want post-round room goal continuation planning", goalProvider.planCalls)
	}
}

func TestRealtimeServicePostRoundWorkReleasesRoomGoalPlanWhenDispatchDefers(t *testing.T) {
	runtimeManager := runtimectx.NewManager()
	goalProvider := &fakeRoomGoalContextProvider{
		stillCurrent: true,
		plan: &protocol.GoalContinuation{
			Goal: protocol.Goal{
				ID:         "goal-room",
				SessionKey: "room:group:conversation-1",
				Status:     protocol.GoalStatusActive,
			},
			RoundID: "goal_continuation_1",
		},
	}
	goalProvider.onPlan = func() {
		runtimeManager.StartRound("room:group:conversation-1", "queued-user-round", nil)
	}
	service := &RealtimeService{
		goals:   goalProvider,
		runtime: runtimeManager,
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
	}

	service.dispatchPostRoundWork(context.Background(), roundValue)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if goalProvider.planCalls != 1 || goalProvider.releaseCalls != 1 {
		t.Fatalf("planCalls=%d releaseCalls=%d, want released deferred room continuation", goalProvider.planCalls, goalProvider.releaseCalls)
	}
}

func TestRealtimeServicePostRoundWorkRecordsRoomGoalFailureWhenDispatchFails(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{
		stillCurrent: true,
		plan: &protocol.GoalContinuation{
			Goal: protocol.Goal{
				ID:         "goal-room",
				SessionKey: "agent:nexus:ws:dm:not-room",
				Status:     protocol.GoalStatusActive,
			},
			RoundID: "goal_continuation_1",
		},
	}
	service := &RealtimeService{
		goals: goalProvider,
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
	}

	service.dispatchPostRoundWork(context.Background(), roundValue)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if goalProvider.planCalls != 1 || len(goalProvider.failures) != 1 {
		t.Fatalf("planCalls=%d failures=%d, want recorded failed room continuation", goalProvider.planCalls, len(goalProvider.failures))
	}
	if !strings.Contains(goalProvider.failures[0], "room goal continuation requires a room session key") {
		t.Fatalf("failure reason = %q, want room session dispatch error", goalProvider.failures[0])
	}
	if goalProvider.releaseCalls != 0 {
		t.Fatalf("releaseCalls=%d, want failed continuation retained as failed", goalProvider.releaseCalls)
	}
}
