// goal_lifecycle_test.go 覆盖 Room Goal 续跑与终态结算测试。
package realtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

func TestRoomContinuationStartAdmissionCancelsRegisteredRootBeforeSlotsRun(t *testing.T) {
	runtimeManager := runtimectx.NewManager()
	provider := &fakeRoomGoalContextProvider{
		startedErr: goalsvc.ErrGoalRevisionStale,
	}
	service := &Service{
		goals:  provider,
		Host:   runtimehost.Host{Runtime: runtimeManager},
		rounds: newRoomRoundRegistry(),
	}
	plan := protocol.GoalContinuation{
		Goal: protocol.Goal{
			ID:         "goal-room-start-admission",
			SessionKey: protocol.BuildRoomSharedSessionKey("conversation-start-admission"),
			Objective:  "old objective", Status: protocol.GoalStatusActive,
			Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(1)},
		},
		RoundID: "room-goal-start-admission", Prompt: "continue",
		HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation",
	}
	provider.onStarted = func() {
		if got := runtimeManager.GetRunningRoundIDs(plan.Goal.SessionKey); !slices.Equal(got, []string{plan.RoundID}) {
			t.Errorf("start admission saw runtime rounds %v, want exact registered root", got)
		}
		if got := runtimeManager.GoalAccountingRoundIDs(plan.Goal.SessionKey, plan.Goal.ID); !slices.Equal(got, []string{"agent-round-lead"}) {
			t.Errorf("start admission saw Goal accounting rounds %v, want exact slot", got)
		}
	}
	registered := make(chan struct{})
	releaseAdmission := make(chan struct{})
	provider.beforeStarted = func() {
		close(registered)
		<-releaseAdmission
	}
	execution := &roomChatExecution{
		service: service,
		ctx:     context.Background(),
		request: ChatRequest{
			SessionKey: plan.Goal.SessionKey, ConversationID: "conversation-start-admission",
			RoundID: plan.RoundID, GoalID: plan.Goal.ID,
			continuationStartAdmission: func(ctx context.Context) error {
				return runtimehost.MarkGoalContinuationStarted(ctx, provider, plan)
			},
		},
		sessionKey:     plan.Goal.SessionKey,
		conversationID: "conversation-start-admission",
	}
	roundValue := &activeRoomRound{
		SessionKey: plan.Goal.SessionKey, ConversationID: execution.conversationID,
		RoundID: plan.RoundID, RootRoundID: plan.RoundID,
		Slots: map[string]*activeRoomSlot{"lead": {
			AgentID: "agent-lead", AgentRoundID: "agent-round-lead",
		}},
		Done: make(chan struct{}),
	}
	startResult := make(chan error, 1)
	go func() { startResult <- execution.startRound(roundValue, nil) }()
	select {
	case <-registered:
	case <-time.After(time.Second):
		t.Fatal("Room continuation did not reach registered start admission")
	}
	if got := roundValue.Slots["lead"].getStatus(); got != "" {
		t.Fatalf("slot started before durable admission: %q", got)
	}
	close(releaseAdmission)
	err := <-startResult
	if !errors.Is(err, goalsvc.ErrGoalRevisionStale) {
		t.Fatalf("startRound() error = %v, want stale admission", err)
	}
	if got := runtimeManager.GetRunningRoundIDs(plan.Goal.SessionKey); len(got) != 0 {
		t.Fatalf("stale admission left running root: %v", got)
	}
	if got := runtimeManager.GoalAccountingRoundIDs(plan.Goal.SessionKey, plan.Goal.ID); len(got) != 0 {
		t.Fatalf("stale admission left Goal accounting identities: %v", got)
	}
	select {
	case <-roundValue.Done:
	case <-time.After(time.Second):
		t.Fatal("stale admission did not close registered root")
	}
	if got := service.rounds.snapshotConversation(execution.conversationID); len(got) != 0 {
		t.Fatalf("stale admission left Room registry entries: %v", got)
	}
	if status := roundValue.Slots["lead"].getStatus(); status != "" {
		t.Fatalf("slot started before durable admission: %q", status)
	}
}

func grantTestRoomGoalAuthority(
	slot *activeRoomSlot,
	sessionKey string,
	goalID string,
) {
	if slot == nil {
		return
	}
	if strings.TrimSpace(sessionKey) == "" {
		sessionKey = slot.RuntimeSessionKey
	}
	slot.grantGoalMutationAuthority(roomGoalMutationAuthority{
		SessionKey:        sessionKey,
		GoalID:            goalID,
		ObjectiveRevision: 1,
		ExecutionID:       "execution-" + strings.TrimSpace(goalID),
		RootRoundID:       goalUsageScopeRoundIDForRoomSlot(slot),
		Source:            roomGoalAuthorityExplicitRound,
	})
}

func attachTestRoomGoalAuthority(roundValue *activeRoomRound, goalID string) {
	if roundValue == nil {
		return
	}
	slot := &activeRoomSlot{
		AgentID:           "agent-goal-test",
		AgentRoundID:      strings.TrimSpace(roundValue.RoundID) + ":agent-goal-test",
		RuntimeSessionKey: strings.TrimSpace(roundValue.SessionKey),
	}
	grantTestRoomGoalAuthority(slot, roundValue.SessionKey, goalID)
	if roundValue.Slots == nil {
		roundValue.Slots = map[string]*activeRoomSlot{}
	}
	roundValue.Slots["agent-goal-test"] = slot
}

func TestBuildRoomGoalCollaborationContextSkipsSingleMemberRoom(t *testing.T) {
	contextValue := buildRoomGoalCollaborationContext(map[string]string{
		"agent-lead": "负责人",
	}, "agent-lead")

	if contextValue != "" {
		t.Fatalf("single-member Room Goal should not require collaboration: %q", contextValue)
	}
}

func TestRealtimeServiceCollaborationCompletionReleasesLiveSourceBarrier(t *testing.T) {
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-live-source")
	binding := &protocol.GoalCollaborationBinding{
		GoalID:            "goal-room",
		ObjectiveRevision: 1,
	}
	sourceSlot := withRoomSlotStatus(&activeRoomSlot{
		AgentID:      "agent-lead",
		AgentRoundID: "goal-continuation-source",
	}, "finished")
	grantTestRoomGoalAuthority(sourceSlot, sessionKey, binding.GoalID)
	sourceSlot.markPendingGoalCollaboration()
	sourceRound := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: "conversation-live-source",
		RoundID:        "goal-continuation-source",
		RootRoundID:    "goal-root-live-source",
		Slots:          map[string]*activeRoomSlot{"agent-lead": sourceSlot},
	}
	targetSlot := withRoomSlotStatus(&activeRoomSlot{
		AgentID:      "agent-peer",
		AgentRoundID: "room-mention-target",
	}, "finished")
	targetSlot.setGoalCollaborationBinding(binding)
	targetSlot.mutable.goal.RememberGoalAssistantMessage(roomGoalTextAssistantMessage(
		"assistant-target",
		"协作结果已完成。",
	))
	targetRound := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: "conversation-live-source",
		RoundID:        "room-mention-target",
		RootRoundID:    sourceRound.RootRoundID,
		Slots:          map[string]*activeRoomSlot{"agent-peer": targetSlot},
	}
	goalProvider := &fakeRoomGoalContextProvider{
		runtimeGoals: map[string]*protocol.Goal{
			sessionKey: {
				ID:         binding.GoalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}
	service := withConstructorDefaults(t, &Service{
		goals: goalProvider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"source": sourceRound,
			"target": targetRound,
		}),
	})

	service.dispatchPostRoundWork(context.Background(), targetRound)

	if sourceSlot.hasPendingGoalCollaboration() {
		t.Fatal("completed target did not release the source Goal collaboration barrier")
	}
	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if goalProvider.planCalls != 1 {
		t.Fatalf("planCalls = %d, want exactly one continuation from target completion", goalProvider.planCalls)
	}
}

func TestMarkActiveGoalCollaborationPendingFindsCrossConversationSource(t *testing.T) {
	binding := &protocol.GoalCollaborationBinding{
		GoalID: "goal-cross-pending", ObjectiveRevision: 3,
	}
	sourceSlot := &activeRoomSlot{AgentID: "agent-lead"}
	otherOwnerSlot := &activeRoomSlot{AgentID: "agent-lead"}
	if !sourceSlot.grantGoalMutationAuthority(roomGoalMutationAuthority{
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-source"),
		GoalID:     binding.GoalID, ObjectiveRevision: binding.ObjectiveRevision,
		RootRoundID: "root-exact", Source: roomGoalAuthorityExplicitRound,
	}) {
		t.Fatal("bind source Goal authority")
	}
	if !otherOwnerSlot.grantGoalMutationAuthority(roomGoalMutationAuthority{
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-other-owner"),
		GoalID:     binding.GoalID, ObjectiveRevision: binding.ObjectiveRevision,
		RootRoundID: "root-exact", Source: roomGoalAuthorityExplicitRound,
	}) {
		t.Fatal("bind other owner Goal authority")
	}
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
		"source": {
			ConversationID: "conversation-source", OwnerUserID: "owner-cross-pending",
			RootRoundID: "root-exact",
			Slots:       map[string]*activeRoomSlot{"lead": sourceSlot},
		},
		"other-owner": {
			ConversationID: "conversation-other-owner", OwnerUserID: "owner-unrelated",
			RootRoundID: "root-exact",
			Slots:       map[string]*activeRoomSlot{"lead": otherOwnerSlot},
		},
	})}

	service.markActiveGoalCollaborationPending(
		"owner-cross-pending", "agent-lead", "root-exact", binding,
	)

	if !sourceSlot.hasPendingGoalCollaboration() {
		t.Fatal("target conversation lookup did not mark the source root pending")
	}
	if otherOwnerSlot.hasPendingGoalCollaboration() {
		t.Fatal("same root/revision in another owner scope was marked pending")
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
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-goal-room",
				},
			},
			RoundID: "goal_continuation_1",
		},
	}
	service := withConstructorDefaults(t, &Service{
		goals: goalProvider,
	})
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
	}
	attachTestRoomGoalAuthority(roundValue, "goal-room")

	service.dispatchPostRoundWork(context.Background(), roundValue)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if goalProvider.planCalls != 1 || len(goalProvider.retryReasons) != 1 {
		t.Fatalf("planCalls=%d retries=%d, want durable retry for pre-runtime room continuation failure", goalProvider.planCalls, len(goalProvider.retryReasons))
	}
	if !strings.Contains(goalProvider.retryReasons[0], "room goal continuation requires a room session key") {
		t.Fatalf("retry reason = %q, want room session dispatch error", goalProvider.retryReasons[0])
	}
	if len(goalProvider.failures) != 0 {
		t.Fatalf("Goal failures = %v, want launch failure isolated to durable receipt", goalProvider.failures)
	}
	if goalProvider.releaseCalls != 0 {
		t.Fatalf("releaseCalls=%d, want failed continuation retained for backoff retry", goalProvider.releaseCalls)
	}
}

// Goal 续接进度测试。

func assertRecordedRoomGoalRoundIDs(t *testing.T, label string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s round IDs = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s round IDs = %v, want %v", label, got, want)
		}
	}
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsFailure(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "agent_round_failure",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	roundValue := &activeRoomRound{
		RootRoundID: "goal_continuation_failure",
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalContinuationProgressForSlot(
		context.Background(),
		slot,
		roundValue,
		exec.RoundExecutionResult{
			TerminalStatus: "error",
			ResultSubtype:  "error",
			ErrorMessage:   "Failed to authenticate. API Error: 401",
		},
		nil,
	)

	failures := goalProvider.recordedFailures()
	if len(failures) != 1 || failures[0] != "Failed to authenticate. API Error: 401" {
		t.Fatalf("failures = %#v, want provider error", failures)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want failure path instead of empty progress", progress)
	}
	assertRecordedRoomGoalRoundIDs(t, "settled receipt", goalProvider.recordedSettledRoundIDs(), "goal_continuation_failure")
	assertRecordedRoomGoalRoundIDs(t, "failure audit", goalProvider.recordedFailureRoundIDs(), "agent_round_failure")
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsCompletionCommandMiss(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "agent_round_completion_miss",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	roundValue := &activeRoomRound{
		RootRoundID: "goal_continuation_completion_miss",
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalContinuationProgressForSlot(
		context.Background(),
		slot,
		roundValue,
		exec.RoundExecutionResult{},
		roomGoalCompletionCommandMissAssistantMessage(),
	)

	misses := goalProvider.recordedCompletionMisses()
	if len(misses) != 1 || !strings.Contains(misses[0], "nexus.command update_goal receipt") {
		t.Fatalf("completion misses = %#v, want one missing update_goal record", misses)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want completion miss path instead of empty progress", progress)
	}
	assertRecordedRoomGoalRoundIDs(t, "settled receipt", goalProvider.recordedSettledRoundIDs(), "goal_continuation_completion_miss")
	assertRecordedRoomGoalRoundIDs(t, "completion-miss audit", goalProvider.recordedCompletionMissRoundIDs(), "agent_round_completion_miss")
}

func TestRecordGoalContinuationProgressForRoomSlotSkipsNoReplyCollaborationEvidence(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "room:group:conversation-1",
		AgentRoundID:      "room_mention_1",
		AgentID:           "agent-peer",
	}
	grantTestRoomGoalAuthority(slot, "room:group:conversation-1", "goal-1")

	service.recordGoalContinuationProgressForSlot(
		context.Background(),
		slot,
		&activeRoomRound{},
		exec.RoundExecutionResult{},
		roomGoalTextAssistantMessage("peer-no-reply", "<nexus_room_no_reply/>"),
	)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.collabEvidence) != 0 {
		t.Fatalf("collaboration evidence = %#v, want no-reply ignored", goalProvider.collabEvidence)
	}
}

func TestRoomGoalUserStopSuspendsContinuationOnEveryTerminalPath(t *testing.T) {
	for _, path := range []string{"provider-acknowledged", "host-cancelled"} {
		t.Run(path, func(t *testing.T) {
			goalProvider := &fakeRoomGoalContextProvider{}
			service := &Service{goals: goalProvider}
			slot := &activeRoomSlot{
				RuntimeSessionKey: "agent:nexus:ws:room:test",
				AgentRoundID:      "agent_round_user_stop",
			}
			grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
			markRoomSlotInterrupted(slot, "")
			roundValue := &activeRoomRound{
				RootRoundID:  "goal_continuation_user_stop",
				InputOptions: sdkprotocol.OutboundMessageOptions{Purpose: "goal_continuation"},
			}

			if path == "provider-acknowledged" {
				service.recordGoalContinuationProgressForSlot(
					context.Background(), slot, roundValue,
					exec.RoundExecutionResult{TerminalStatus: "finished", ResultSubtype: "interrupted"}, nil,
				)
			} else {
				service.recordGoalSlotInterrupted(context.Background(), slot, roundValue)
			}

			goalProvider.mu.Lock()
			stops := append([]string(nil), goalProvider.userStopRoundIDs...)
			goalProvider.mu.Unlock()
			assertRecordedRoomGoalRoundIDs(t, "user stop audit", stops, "agent_round_user_stop")
			assertRecordedRoomGoalRoundIDs(t, "settled receipt", goalProvider.recordedSettledRoundIDs(), "goal_continuation_user_stop")
			if progress := goalProvider.recordedProgress(); len(progress) != 0 {
				t.Fatalf("progress = %#v, want a user stop never counted as progress", progress)
			}
		})
	}
}

func TestRoomGoalNonUserCancellationOnlySettlesContinuationReceipt(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "agent_round_paused_member",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	markRoomSlotInterrupted(slot, "成员已暂停参与")
	roundValue := &activeRoomRound{
		RootRoundID:  "goal_continuation_paused_member",
		InputOptions: sdkprotocol.OutboundMessageOptions{Purpose: "goal_continuation"},
	}

	service.recordGoalSlotInterrupted(context.Background(), slot, roundValue)

	goalProvider.mu.Lock()
	stops := len(goalProvider.userStopRoundIDs)
	goalProvider.mu.Unlock()
	if stops != 0 {
		t.Fatalf("user stops = %d, want member pause treated as a non-user cancellation", stops)
	}
	assertRecordedRoomGoalRoundIDs(t, "settled receipt", goalProvider.recordedSettledRoundIDs(), "goal_continuation_paused_member")
}

func TestRoomSubagentUsageRetryDoesNotFinalizeWhileParentRuns(t *testing.T) {
	const goalID = "goal-source-running-parent"
	slot := &activeRoomSlot{
		AgentID:           "agent-running-parent",
		AgentRoundID:      "round-running-parent",
		RuntimeSessionKey: "agent:running-parent",
	}
	slot.setRuntimeKind("nxs")
	slot.setGoalBinding("room:group:running-parent", goalID)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	slot.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:  "room:group:running-parent",
		RoundID:     "room-round-running-parent",
		OwnerUserID: "owner-running-parent",
		Slots:       map[string]*activeRoomSlot{"worker": slot},
	}
	roundValue.RunningSubagents.Store(true)
	provider := &retryingRoomGoalUsageSourceProvider{
		fakeRoomGoalUsageFinalizer: &fakeRoomGoalUsageFinalizer{
			fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: roundValue.SessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		persisted: make(chan struct{}),
	}
	service := &Service{goals: provider}
	accelerateRoomGoalUsageRetry(service)
	slot.markSubagentUsagePending("task-running-parent", 10)
	service.startRoomSubagentUsageRetry(roundValue, slot)

	select {
	case <-provider.persisted:
	case <-time.After(time.Second):
		t.Fatal("background Room usage retry did not persist running-parent child")
	}
	waitForRoomGoalUsageRetryStopped(t, slot)
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("child retry finalized shared Goal while parent was running: %d calls", calls)
	}
	if !roundValue.RunningSubagents.Load() {
		t.Fatal("child retry released post-round state while parent was running")
	}
}

func TestRoomParentUsageRetryRecoversWithoutChildOrRuntimeMessage(t *testing.T) {
	const (
		conversationID = "conversation-parent-background-retry"
		goalID         = "goal-parent-background-retry"
	)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	slot := &activeRoomSlot{
		AgentID:      "agent-parent-background-retry",
		AgentRoundID: "round-parent-background-retry",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(
			conversationID,
			"agent-parent-background-retry",
			protocol.RoomTypeGroup,
		),
	}
	grantTestRoomGoalAuthority(slot, sessionKey, goalID)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	slot.setStatus("finished")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round-parent-background-retry",
		OwnerUserID:    "owner-parent-background-retry",
		Slots:          map[string]*activeRoomSlot{"worker": slot},
	}
	base := &fakeRoomGoalContextProvider{}
	provider := &retryingRoomGoalUsageSourceProvider{
		fakeRoomGoalUsageFinalizer: &fakeRoomGoalUsageFinalizer{
			fakeRoomGoalContextProvider: base,
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		parentUsageFailuresRemaining: runtimehost.GoalUsagePersistAttempts * 2,
		finalizeFailuresRemaining:    runtimehost.GoalUsagePersistAttempts,
		parentUsagePersisted:         make(chan struct{}),
	}
	service := withConstructorDefaults(t, &Service{
		goals: provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			roundValue.RoundID: roundValue,
		}),
	})
	accelerateRoomGoalUsageRetry(service)
	result := exec.RoundExecutionResult{Usage: sdkprotocol.TokenUsage{
		InputTokens:  90,
		OutputTokens: 10,
		TotalTokens:  100,
	}}

	// 模拟 slot 自身 terminal settlement 的同步窗口先耗尽。
	service.finalizeGoalUsageForSlot(context.Background(), slot, result, nil)
	if slot.goalUsageTerminalSettled() {
		t.Fatal("terminal parent usage unexpectedly settled inside the first retry window")
	}
	if service.settleCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("round settlement unexpectedly succeeded inside the second retry window")
	}
	if !roundValue.RunningSubagents.Load() {
		t.Fatal("failed parent settlement did not establish the post-round barrier")
	}

	select {
	case <-provider.parentUsagePersisted:
	case <-time.After(3 * time.Second):
		t.Fatal("background Room parent usage retry did not recover without another runtime message")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		base.mu.Lock()
		planCalls := base.planCalls
		base.mu.Unlock()
		if provider.finalizeCallCount() == 1 &&
			!roundValue.RunningSubagents.Load() &&
			roundValue.postRoundDispatched.Load() &&
			planCalls == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"background parent recovery finalize_calls=%d barrier=%v post_round=%v plan_calls=%d",
				provider.finalizeCallCount(),
				roundValue.RunningSubagents.Load(),
				roundValue.postRoundDispatched.Load(),
				planCalls,
			)
		}
		time.Sleep(time.Millisecond)
	}

	if !slot.goalUsageTerminalSettled() {
		t.Fatal("background parent retry left terminal usage unsettled")
	}
	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].ActualTokens() != 100 || usages[0].BudgetTokens() != 100 {
		t.Fatalf("persisted parent usage = %#v, want exact 100 once", usages)
	}
	if attempts := provider.parentUsageAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts*2+1 {
		t.Fatalf(
			"parent usage attempts = %d, want %d failures plus one background success",
			attempts,
			runtimehost.GoalUsagePersistAttempts*2,
		)
	}
	if attempts := provider.finalizeAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts+1 {
		t.Fatalf(
			"shared finalization attempts = %d, want %d failures plus one background success",
			attempts,
			runtimehost.GoalUsagePersistAttempts,
		)
	}

	// runRound 与 worker 即使在 barrier 释放处竞争，也只能派发一次。
	service.dispatchPostRoundWorkOnce(context.Background(), roundValue)
	base.mu.Lock()
	planCalls := base.planCalls
	base.mu.Unlock()
	if planCalls != 1 {
		t.Fatalf("post-round plan calls = %d, want exactly once", planCalls)
	}
}

type fakeRoomGoalUsageFinalizer struct {
	*fakeRoomGoalContextProvider
	report                 protocol.GoalUsageReport
	finalizeCalls          int
	beforeFinalize         func()
	claimFailuresRemaining int
	claimAttempts          int
	claimSuccesses         int
}

type retryingRoomGoalUsageSourceProvider struct {
	*fakeRoomGoalUsageFinalizer
	sourceMu                     sync.Mutex
	failuresRemaining            int
	parentUsageFailuresRemaining int
	parentUsageAttempts          int
	finalizeFailuresRemaining    int
	finalizeAttempts             int
	persistedTotal               int64
	persisted                    chan struct{}
	persistedOnce                sync.Once
	parentUsagePersisted         chan struct{}
	parentUsagePersistedOnce     sync.Once
}

func (p *retryingRoomGoalUsageSourceProvider) RecordUsageForGoal(
	ctx context.Context,
	goalID string,
	usage protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.sourceMu.Lock()
	p.parentUsageAttempts++
	if p.parentUsageFailuresRemaining > 0 {
		p.parentUsageFailuresRemaining--
		p.sourceMu.Unlock()
		return nil, errors.New("injected Room parent usage persistence failure")
	}
	p.sourceMu.Unlock()
	updated, err := p.fakeRoomGoalUsageFinalizer.RecordUsageForGoal(ctx, goalID, usage, roundID)
	if err == nil && p.parentUsagePersisted != nil {
		p.parentUsagePersistedOnce.Do(func() { close(p.parentUsagePersisted) })
	}
	return updated, err
}

func (p *retryingRoomGoalUsageSourceProvider) parentUsageAttemptCount() int {
	p.sourceMu.Lock()
	defer p.sourceMu.Unlock()
	return p.parentUsageAttempts
}

func (p *retryingRoomGoalUsageSourceProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.sourceMu.Lock()
	if p.failuresRemaining > 0 {
		p.failuresRemaining--
		p.sourceMu.Unlock()
		return protocol.GoalUsageSourceResult{}, errors.New("injected Room source persistence failure")
	}
	p.persistedTotal = snapshot.CumulativeActualTokens
	p.sourceMu.Unlock()
	p.persistedOnce.Do(func() { close(p.persisted) })
	return protocol.GoalUsageSourceResult{}, nil
}

func waitForRoomGoalUsageRetryStopped(t *testing.T, slot *activeRoomSlot) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		slot.mutable.goal.Mu.RLock()
		running := slot.mutable.goal.UsageRetrying
		slot.mutable.goal.Mu.RUnlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for Room Goal usage retry worker")
}

func (p *retryingRoomGoalUsageSourceProvider) FinalizeUsageForGoal(
	ctx context.Context,
	goalID string,
	delta protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.sourceMu.Lock()
	p.finalizeAttempts++
	if p.finalizeFailuresRemaining > 0 {
		p.finalizeFailuresRemaining--
		p.sourceMu.Unlock()
		return nil, errors.New("injected Room Goal finalization failure")
	}
	p.sourceMu.Unlock()
	return p.fakeRoomGoalUsageFinalizer.FinalizeUsageForGoal(ctx, goalID, delta, roundID)
}

func (p *retryingRoomGoalUsageSourceProvider) finalizeAttemptCount() int {
	p.sourceMu.Lock()
	defer p.sourceMu.Unlock()
	return p.finalizeAttempts
}

func (p *fakeRoomGoalUsageFinalizer) UsageByGoalID(
	_ context.Context,
	_ string,
) (*protocol.GoalUsageReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	report := p.report
	return &report, nil
}

func (p *fakeRoomGoalUsageFinalizer) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	beforeFinalize := p.beforeFinalize
	p.mu.Unlock()
	if beforeFinalize != nil {
		beforeFinalize()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func (p *fakeRoomGoalUsageFinalizer) ClaimUsageSourceRound(
	_ context.Context,
	_ protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claimAttempts++
	if p.claimFailuresRemaining > 0 {
		p.claimFailuresRemaining--
		return protocol.GoalUsageSourceResult{}, errors.New("injected Room usage claim failure")
	}
	p.claimSuccesses++
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *fakeRoomGoalUsageFinalizer) finalizeCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.finalizeCalls
}
