package dm

import (
	"context"
	"slices"
	"testing"
	"time"

	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// A user stop on a running Goal continuation must suspend continuation and
// settle its launch receipt on every stop path: an exact-round stop and a
// session stop that the provider acknowledges, and a provider that never
// answers so the host cancels locally. A host control stop (Goal pause,
// automation cancel) only settles the receipt and never pauses the queue.
func TestServiceUserStopOnGoalContinuationRecordsUserStop(t *testing.T) {
	for _, mode := range []string{"exact", "session", "silent-provider", "host-control"} {
		t.Run(mode, func(t *testing.T) {
			cfg := newDMTestConfig(t)
			migrateDMSQLite(t, cfg.DatabaseURL)
			agentService := newDMAgentService(t, cfg)
			client := newFakeDMClient()
			queryStarted := make(chan struct{}, 1)
			client.onQuery = func(context.Context, string) { queryStarted <- struct{}{} }
			client.onInterrupt = func(context.Context) {
				if mode == "silent-provider" {
					return
				}
				go func() {
					client.messages <- sdkprotocol.ReceivedMessage{
						Type:      sdkprotocol.MessageTypeResult,
						SessionID: client.sessionID,
						UUID:      "result-user-stop-" + mode,
						Result: &sdkprotocol.ResultMessage{
							Subtype: "interrupted", DurationMS: 1, NumTurns: 1,
							Result: messagepkg.InterruptWithoutMessage,
						},
					}
				}()
			}
			runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
			service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
			sessionKey := "agent:nexus:ws:dm:user-stop-" + mode
			t.Cleanup(func() { _ = runtimeManager.CloseSession(context.Background(), sessionKey) })
			goal := protocol.Goal{
				ID: "goal-user-stop", SessionKey: sessionKey, Objective: "stop means stop", Status: protocol.GoalStatusActive,
				Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(1)},
			}
			plan := protocol.GoalContinuation{
				Goal: goal, RoundID: "goal_continuation_user_stop_" + mode, Prompt: "continue",
				HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation",
			}
			goalProvider := &fakeGoalContextProvider{reservation: true, continuationCount: 1, runtimeContext: "ctx", runtimeGoal: &goal}
			service.SetGoalContextProvider(goalProvider)

			if err := service.DispatchGoalContinuation(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			select {
			case <-queryStarted:
			case <-time.After(2 * time.Second):
				t.Fatal("continuation did not start")
			}
			request := InterruptRequest{SessionKey: sessionKey, RoundID: plan.RoundID, UserStop: mode != "host-control"}
			if mode == "session" {
				request.RoundID = ""
			}
			if err := service.HandleInterrupt(context.Background(), request); err != nil {
				t.Fatalf("interrupt: %v", err)
			}
			waitForDMRuntimeIdle(t, runtimeManager, sessionKey)
			deadline := time.Now().Add(2 * time.Second)
			for {
				goalProvider.mu.Lock()
				stops := slices.Clone(goalProvider.userStopRoundIDs)
				revisions := slices.Clone(goalProvider.userStopRevisions)
				progress := len(goalProvider.progress)
				failures := len(goalProvider.failures)
				settled := goalProvider.settledCalls
				goalProvider.mu.Unlock()
				if mode == "host-control" && (settled > 0 || time.Now().After(deadline)) {
					if len(stops) != 0 || progress != 0 || failures != 0 || settled == 0 {
						t.Fatalf("host control stop: stops=%v progress=%d failures=%d settled=%d, want receipt settled only", stops, progress, failures, settled)
					}
					_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
					if err != nil {
						t.Fatal(err)
					}
					if paused, err := service.InputQueue.DispatchPaused(location); err != nil || paused {
						t.Fatalf("host control stop paused the queue: paused=%v err=%v", paused, err)
					}
					return
				}
				if mode != "host-control" && (len(stops) > 0 || time.Now().After(deadline)) {
					if !slices.Equal(stops, []string{plan.RoundID}) || !slices.Equal(revisions, []int64{1}) {
						t.Fatalf("user stops = %v revisions = %v, want exactly the stopped round at revision 1", stops, revisions)
					}
					if progress != 0 || failures != 0 {
						t.Fatalf("progress=%d failures=%d, want a user stop never counted as progress or failure", progress, failures)
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func TestRoundRunnerNonUserInterruptOnlySettlesContinuationReceipt(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:non-user-interrupt",
		roundID:        "goal_continuation_non_user",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-1"},
		inputOptions:   sdkprotocol.OutboundMessageOptions{Purpose: "goal_continuation"},
	}

	runner.recordGoalRoundInterrupted("收到新的用户消息，上一轮已停止")

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if goalProvider.settledCalls != 1 || len(goalProvider.userStopRoundIDs) != 0 || len(goalProvider.progress) != 0 {
		t.Fatalf("settled=%d stops=%v progress=%v, want receipt settled without a user stop", goalProvider.settledCalls, goalProvider.userStopRoundIDs, goalProvider.progress)
	}
}
