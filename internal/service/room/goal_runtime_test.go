package room

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeRoomGoalContextProvider struct {
	mu    sync.Mutex
	usage []protocol.GoalUsage
}

func (p *fakeRoomGoalContextProvider) RuntimeContext(context.Context, string) (string, *protocol.Goal, error) {
	return "", nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordUsageForSession(_ context.Context, _ string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordUsageForGoal(_ context.Context, _ string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	return nil, nil
}

func TestRecordGoalUsageForRoomSlotUsesToolCompletionDelta(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
		GoalIDForUsage:    "goal-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(true),
	}

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  6,
			OutputTokens: 3,
			TotalTokens:  9,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[0].InputTokens != 4 || usages[0].OutputTokens != 1 || usages[0].Total() != 5 {
		t.Fatalf("first usage = %#v, want 4/1", usages[0])
	}
	if usages[1].InputTokens != 2 || usages[1].OutputTokens != 2 || usages[1].Total() != 4 {
		t.Fatalf("second usage = %#v, want remaining 2/2", usages[1])
	}
}

func TestRecordGoalUsageForRoomSlotUsesAssistantSnapshotOnAbort(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
		GoalIDForUsage:    "goal-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(true),
	}

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{}, roomGoalAssistantUsageMessage(9, 4))

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[1].InputTokens != 5 || usages[1].OutputTokens != 3 || usages[1].Total() != 8 {
		t.Fatalf("abort usage = %#v, want remaining 5/3", usages[1])
	}
}

func TestRegisterSlotGoalRuntimeMakesGoalGuidanceQueueable(t *testing.T) {
	manager := runtimectx.NewManager()
	service := &RealtimeService{runtime: manager}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:conversation-1:agent-1",
		AgentRoundID:      "room-round-1:agent-1",
	}

	cleanup := service.registerSlotGoalRuntime(slot)
	roundIDs, err := manager.QueueGuidanceInput(context.Background(), slot.RuntimeSessionKey, "goal-event-1", "budget reached")
	if err != nil {
		t.Fatalf("QueueGuidanceInput() error = %v", err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("roundIDs = %#v, want slot round", roundIDs)
	}
	if count := manager.PendingGuidanceCount(slot.RuntimeSessionKey); count != 1 {
		t.Fatalf("PendingGuidanceCount = %d, want 1", count)
	}

	cleanup()
	if _, err := manager.QueueGuidanceInput(context.Background(), slot.RuntimeSessionKey, "goal-event-2", "late guidance"); !errors.Is(err, runtimectx.ErrNoRunningRound) {
		t.Fatalf("QueueGuidanceInput() after cleanup error = %v, want ErrNoRunningRound", err)
	}
}

func (p *fakeRoomGoalContextProvider) recordedUsage() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.usage...)
}

func roomGoalToolResultAssistantMessage(
	toolUseID string,
	toolName string,
	inputTokens int64,
	outputTokens int64,
) protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
		"content": []map[string]any{
			{"type": "tool_use", "id": toolUseID, "name": toolName},
			{"type": "tool_result", "tool_use_id": toolUseID},
		},
	}
}

func roomGoalAssistantUsageMessage(inputTokens int64, outputTokens int64) protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
}
