package room

import (
	"context"
	"testing"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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

func TestRoomSlotRecordsUsageToSharedGoalAfterCreateGoalTool(t *testing.T) {
	for _, toolName := range []string{"create_goal", "mcp__nexus_goal__create_goal"} {
		t.Run(toolName, func(t *testing.T) {
			sharedSessionKey := "room:group:conversation-1"
			goalProvider := &fakeRoomGoalContextProvider{}
			service := &RealtimeService{goals: goalProvider}
			slot := &activeRoomSlot{
				RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
				GoalSessionKey:    sharedSessionKey,
				AgentRoundID:      "round-1:agent-1",
				GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(false),
			}

			service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", toolName, 4, 1))
			service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
				Usage: sdkprotocol.TokenUsage{
					InputTokens:  9,
					OutputTokens: 3,
					TotalTokens:  12,
				},
			}, nil)

			usages := goalProvider.recordedUsage()
			if len(usages) != 1 {
				t.Fatalf("len(usages) = %d, want post-create delta", len(usages))
			}
			if usages[0].InputTokens != 5 || usages[0].OutputTokens != 2 || usages[0].Total() != 7 {
				t.Fatalf("usage = %#v, want 5/2 delta after create_goal baseline", usages[0])
			}
			if len(goalProvider.usageSessionKeys) != 1 || goalProvider.usageSessionKeys[0] != sharedSessionKey {
				t.Fatalf("usageSessionKeys = %#v, want shared room goal session", goalProvider.usageSessionKeys)
			}
		})
	}
}
