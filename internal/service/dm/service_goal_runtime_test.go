package dm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoundRunnerRecordsGoalUsageAtToolCompletion(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 3))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[0].InputTokens != 4 || usages[0].OutputTokens != 3 || usages[0].Total() != 7 {
		t.Fatalf("first usage = %#v, want 4/3", usages[0])
	}
	if usages[1].InputTokens != 6 || usages[1].OutputTokens != 2 || usages[1].Total() != 8 {
		t.Fatalf("second usage = %#v, want remaining 6/2", usages[1])
	}
}

func TestRoundRunnerRecordsAbortGoalUsageFromAssistantSnapshot(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{}, goalAssistantUsageMessage(7, 3))

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[1].InputTokens != 3 || usages[1].OutputTokens != 2 || usages[1].Total() != 5 {
		t.Fatalf("abort usage = %#v, want remaining 3/2", usages[1])
	}
}

func TestRoundRunnerMarksUsageLimitAfterAccounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  3,
			OutputTokens: 2,
			TotalTokens:  5,
		},
		UsageLimitReached: true,
		UsageLimitReason:  "You've hit your usage limit.",
	}, nil)
	runner.recordGoalUsageLimit(runtimectx.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "You've hit your usage limit.",
	})

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 || usages[0].Total() != 5 {
		t.Fatalf("usages = %#v, want usage recorded before limit", usages)
	}
	reasons := goalProvider.recordedUsageLimitReasons()
	if len(reasons) != 1 || reasons[0] != "You've hit your usage limit." {
		t.Fatalf("usage limit reasons = %#v, want runtime reason", reasons)
	}
}

func TestRoundRunnerRecordsEmptyGoalContinuationProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || progress[0] {
		t.Fatalf("progress = %#v, want one false continuation progress", progress)
	}
}

func TestRoundRunnerSkipsEmptyGoalContinuationProgressWhileSubagentRuns(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		subagentTasks:  map[string]struct{}{"task-1": {}},
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want running subagent to defer empty continuation progress", progress)
	}
}

func TestRoundRunnerRecordsGoalContinuationFailure(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{
		TerminalStatus: "error",
		ResultSubtype:  "error",
		ErrorMessage:   "Failed to authenticate. API Error: 401",
	})

	failures := goalProvider.recordedFailures()
	if len(failures) != 1 || failures[0] != "Failed to authenticate. API Error: 401" {
		t.Fatalf("failures = %#v, want provider error", failures)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want failure path instead of empty progress", progress)
	}
}

func TestRoundRunnerRecordsGoalContinuationToolProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || !progress[0] {
		t.Fatalf("progress = %#v, want one true continuation progress", progress)
	}
}

func TestRoundRunnerRecordsGoalCompletionToolMiss(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}
	runner.rememberGoalAssistantMessage(goalCompletionToolMissAssistantMessage())

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	misses := goalProvider.recordedCompletionMisses()
	if len(misses) != 1 || !strings.Contains(misses[0], "mcp__nexus_goal__update_goal") {
		t.Fatalf("completion misses = %#v, want one missing update_goal record", misses)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want completion miss path instead of empty progress", progress)
	}
}

func TestRoundRunnerRecordsUserGoalActivityInsteadOfContinuationProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-user",
		goalIDForUsage: "goal-1",
	}

	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.activities) != 1 || goalProvider.activities[0] != "round-user" {
		t.Fatalf("activities = %#v, want explicit goal activity", goalProvider.activities)
	}
	if len(goalProvider.progress) != 0 {
		t.Fatalf("progress = %#v, want no continuation progress for user round", goalProvider.progress)
	}
}

func TestRoundRunnerClosesGoalUsageAfterUpdateGoal(t *testing.T) {
	for _, toolName := range []string{"update_goal", "mcp__nexus_goal__update_goal"} {
		t.Run(toolName, func(t *testing.T) {
			goalProvider := &fakeGoalContextProvider{}
			runner := &roundRunner{
				service:        &Service{goals: goalProvider},
				sessionKey:     "agent:nexus:ws:dm:test",
				roundID:        "round-1",
				goalIDForUsage: "goal-1",
				goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
			}

			runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", toolName, false, 10, 2))
			runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
				Usage: sdkprotocol.TokenUsage{
					InputTokens:  20,
					OutputTokens: 5,
					TotalTokens:  25,
				},
			}, nil)

			usages := goalProvider.recordedUsage()
			if len(usages) != 1 {
				t.Fatalf("len(usages) = %d, want 1", len(usages))
			}
			if usages[0].InputTokens != 10 || usages[0].OutputTokens != 2 || usages[0].Total() != 12 {
				t.Fatalf("usage = %#v, want update_goal usage only", usages[0])
			}
		})
	}
}

func TestRoundRunnerClearGoalUsageStopsLaterAccounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.clearGoalUsage()
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  20,
			OutputTokens: 5,
			TotalTokens:  25,
		},
	}, nil)

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("usages = %#v, want none after clear", usages)
	}
}

func TestRoundRunnerActivateGoalUsageRestartsFromCurrentSnapshot(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.clearGoalUsage()
	runner.rememberGoalAssistantMessage(goalToolResultAssistantMessage("tool-2", "read_file", false, 7, 3))
	if err := runner.activateGoalUsage(context.Background()); err != nil {
		t.Fatal(err)
	}
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want initial usage and post-activate delta", len(usages))
	}
	if usages[1].InputTokens != 3 || usages[1].OutputTokens != 2 || usages[1].Total() != 5 {
		t.Fatalf("post-activate usage = %#v, want 3/2", usages[1])
	}
}

func TestRoundRunnerResetsGoalUsageAfterCreateGoal(t *testing.T) {
	for _, toolName := range []string{"create_goal", "mcp__nexus_goal__create_goal"} {
		t.Run(toolName, func(t *testing.T) {
			goalProvider := &fakeGoalContextProvider{}
			runner := &roundRunner{
				service:    &Service{goals: goalProvider},
				sessionKey: "agent:nexus:ws:dm:test",
				roundID:    "round-1",
				goalUsage:  goalsvc.NewRuntimeUsageAccumulator(false),
			}

			runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", toolName, false, 5, 1))
			runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
				Usage: sdkprotocol.TokenUsage{
					InputTokens:  8,
					OutputTokens: 3,
					TotalTokens:  11,
				},
			}, nil)

			usages := goalProvider.recordedUsage()
			if len(usages) != 1 {
				t.Fatalf("len(usages) = %d, want 1", len(usages))
			}
			if usages[0].InputTokens != 3 || usages[0].OutputTokens != 2 || usages[0].Total() != 5 {
				t.Fatalf("usage = %#v, want post-create delta 3/2", usages[0])
			}
		})
	}
}

func TestRoundRunnerIgnoresGoalRuntimeInPlanMode(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:          &Service{goals: goalProvider},
		sessionKey:       "agent:nexus:ws:dm:test-goal-plan-runtime",
		roundID:          "round-plan",
		goalIDForUsage:   "goal-plan",
		goalUsage:        goalsvc.NewRuntimeUsageAccumulator(true),
		goalUsageStarted: time.Now(),
		permissionMode:   sdkpermission.ModePlan,
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 2,
		},
		ElapsedTimeSeconds: 3,
	}, protocol.Message{})
	runner.recordGoalUsageLimit(runtimectx.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "usage limit",
	})
	runner.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{})

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("plan mode recorded goal usage: %#v", usages)
	}
	if reasons := goalProvider.recordedUsageLimitReasons(); len(reasons) != 0 {
		t.Fatalf("plan mode recorded usage limit: %#v", reasons)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("plan mode recorded continuation progress: %#v", progress)
	}
}
