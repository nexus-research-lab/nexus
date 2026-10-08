package realtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoomContinuationStartAdmissionCancelsRegisteredRootBeforeSlotsRun(t *testing.T) {
	runtimeManager := runtimectx.NewManager()
	provider := &fakeRoomGoalContextProvider{
		startedErr: goalsvc.ErrGoalRevisionStale,
	}
	service := &Service{
		goals:   provider,
		runtime: runtimeManager,
		rounds:  newRoomRoundRegistry(),
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
				return markRoomGoalContinuationStarted(ctx, provider, plan)
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

func TestRoomGoalMutationAuthorityConcurrentRejectedGrantsPreserveSharedState(t *testing.T) {
	base := roomGoalMutationAuthority{
		SessionKey:        "room:group:conversation-1",
		GoalID:            "goal-room",
		ObjectiveRevision: 1,
		ExecutionID:       "execution-room",
		RootRoundID:       "round-1",
		Source:            roomGoalAuthorityExplicitRound,
	}
	slot := &activeRoomSlot{}
	if !slot.grantGoalMutationAuthority(base) {
		t.Fatal("grant initial Room Goal authority")
	}

	start := make(chan struct{})
	results := make(chan bool, 2)
	var wait sync.WaitGroup
	for _, revision := range []int64{2, 3} {
		candidate := base
		candidate.ObjectiveRevision = revision
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- slot.grantGoalMutationAuthority(candidate)
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for granted := range results {
		if granted {
			t.Fatal("concurrent incompatible Room Goal authority was granted")
		}
	}
	if got := slot.goalMutationAuthority(); got != base {
		t.Fatalf("fixed authority after concurrent rejections = %+v, want %+v", got, base)
	}
	shared, ok := slot.ensureGoalAuthorityState().Load()
	if !ok || shared.GoalID != base.GoalID ||
		shared.ObjectiveRevision != base.ObjectiveRevision ||
		shared.ExecutionID != base.ExecutionID {
		t.Fatalf("shared authority after concurrent rejections = %+v, ok=%t, want %+v", shared, ok, base)
	}
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

func TestBuildRoomGoalCollaborationContextKeepsCollaborationOptional(t *testing.T) {
	contextValue := buildRoomGoalCollaborationContext(map[string]string{
		"agent-lead":  "负责人",
		"agent-alpha": "Alpha",
		"agent-beta":  "Beta",
	}, "agent-lead")

	for _, expected := range []string{
		"Room Goal collaboration options",
		"Lead agent for this continuation: 负责人 (agent_id=agent-lead)",
		"@Alpha (agent_id=agent-alpha)",
		"@Beta (agent_id=agent-beta)",
		"assess task complexity, separable work, member fit",
		"@mention is conversation-only",
		"substantive public reply",
		"audit context, not a completion gate",
		"use assign_work for one distinct Ready Work Item",
		"do not duplicate that deliverable",
		"coordination, unblocking, integration, and verification",
		"explicitly cancel that work first",
	} {
		if !strings.Contains(contextValue, expected) {
			t.Fatalf("collaboration context missing %q:\n%s", expected, contextValue)
		}
	}
	if strings.Contains(contextValue, "@负责人") {
		t.Fatalf("collaboration context should not delegate to lead:\n%s", contextValue)
	}
	for _, forbidden := range []string{"Visible collaboration is a required part", "Completion requires room-visible collaborator evidence"} {
		if strings.Contains(contextValue, forbidden) {
			t.Fatalf("collaboration context must not contain completion gate %q:\n%s", forbidden, contextValue)
		}
	}
}

func TestBuildRoomGoalCollaborationContextSkipsSingleMemberRoom(t *testing.T) {
	contextValue := buildRoomGoalCollaborationContext(map[string]string{
		"agent-lead": "负责人",
	}, "agent-lead")

	if contextValue != "" {
		t.Fatalf("single-member Room Goal should not require collaboration: %q", contextValue)
	}
}

func TestRealtimeServicePostRoundWorkRejectsStaleCollaborationAttribution(t *testing.T) {
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-stale-collaboration")
	goalProvider := &fakeRoomGoalContextProvider{
		runtimeGoals: map[string]*protocol.Goal{
			sessionKey: {
				ID:         "goal-room",
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusActive,
				Metadata: map[string]any{
					protocol.GoalMetadataObjectiveRevision: int64(2),
				},
			},
		},
	}
	service := &Service{goals: goalProvider}
	slot := withRoomSlotStatus(&activeRoomSlot{
		AgentID:      "agent-peer",
		AgentRoundID: "room-mention-peer-stale",
	}, "finished")
	slot.setGoalCollaborationBinding(&protocol.GoalCollaborationBinding{
		GoalID:            "goal-room",
		ObjectiveRevision: 1,
	})
	slot.rememberGoalAssistantMessage(roomGoalTextAssistantMessage(
		"assistant-peer-stale",
		"这是旧目标的结果。",
	))

	service.dispatchPostRoundWork(context.Background(), &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: "conversation-stale-collaboration",
		RoundID:        "room-mention-round-stale",
		Slots:          map[string]*activeRoomSlot{"agent-peer": slot},
	})

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.collabEvidence) != 0 ||
		len(goalProvider.activities) != 0 ||
		len(goalProvider.handbacks) != 0 ||
		goalProvider.planCalls != 0 {
		t.Fatalf(
			"stale collaboration mutated Goal: evidence=%#v activities=%#v handbacks=%#v planCalls=%d",
			goalProvider.collabEvidence,
			goalProvider.activities,
			goalProvider.handbacks,
			goalProvider.planCalls,
		)
	}
}

func TestRealtimeServicePostRoundWorkReturnsControlAfterNoReplyWithoutClaimingEvidence(t *testing.T) {
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-no-reply-collaboration")
	goalProvider := &fakeRoomGoalContextProvider{
		runtimeGoals: map[string]*protocol.Goal{
			sessionKey: {
				ID:         "goal-room",
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}
	service := withConstructorDefaults(t, &Service{goals: goalProvider})
	slot := withRoomSlotStatus(&activeRoomSlot{
		AgentID:      "agent-peer",
		AgentRoundID: "room-mention-peer-no-reply",
	}, "finished")
	slot.setGoalCollaborationBinding(&protocol.GoalCollaborationBinding{
		GoalID:            "goal-room",
		ObjectiveRevision: 1,
	})
	slot.rememberGoalAssistantMessage(roomGoalTextAssistantMessage(
		"assistant-peer-no-reply",
		"<nexus_room_no_reply/>",
	))

	service.dispatchPostRoundWork(context.Background(), &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: "conversation-no-reply-collaboration",
		RoundID:        "room-mention-round-no-reply",
		Slots:          map[string]*activeRoomSlot{"agent-peer": slot},
	})

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.collabEvidence) != 0 || len(goalProvider.activities) != 0 ||
		len(goalProvider.handbacks) != 1 ||
		goalProvider.handbacks[0] != "room-mention-round-no-reply" {
		t.Fatalf(
			"no-reply handback mismatch: evidence=%#v activities=%#v handbacks=%#v",
			goalProvider.collabEvidence,
			goalProvider.activities,
			goalProvider.handbacks,
		)
	}
	if goalProvider.planCalls != 1 {
		t.Fatalf("planCalls = %d, want control returned to Goal continuation", goalProvider.planCalls)
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
	targetSlot.rememberGoalAssistantMessage(roomGoalTextAssistantMessage(
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

func TestRoomGoalCollaborationDurableFenceSurvivesRestart(t *testing.T) {
	const conversationID = "conversation-durable-collaboration"
	const ownerUserID = "owner-durable-collaboration"
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	store := workspacestore.NewRoomPublicHandoffStore(stateRoot)
	handoff := workspacestore.RoomPublicHandoff{
		HandoffID:       "handoff-durable-goal",
		ConversationID:  conversationID,
		RootRoundID:     "goal-root-durable",
		SourceMessageID: "assistant-source-durable",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-peer",
		Content:         "请核对",
		QueueSource:     protocol.InputQueueSourceAgentPublicMention,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID:            "goal-room",
			ObjectiveRevision: 1,
		},
	}
	if _, _, err := store.Detect(ownerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	goalProvider := &fakeRoomGoalContextProvider{
		runtimeGoals: map[string]*protocol.Goal{
			sessionKey: {
				ID:         "goal-room",
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:          "room-durable-collaboration",
			OwnerUserID: ownerUserID,
		},
		Conversation: protocol.ConversationRecord{ID: conversationID},
	}
	service := withConstructorDefaults(t, &Service{goals: goalProvider, publicHandoffs: store})

	if !service.shouldDeferGoalContinuationForTargetStateLocked(
		context.Background(),
		sessionKey,
		contextValue,
	) {
		t.Fatal("durable Goal collaboration edge did not defer continuation after restart")
	}
	if err := store.MarkTerminal(ownerUserID, conversationID, handoff.HandoffID, "finished"); err != nil {
		t.Fatal(err)
	}
	if service.shouldDeferGoalContinuationForTargetStateLocked(
		context.Background(),
		sessionKey,
		contextValue,
	) {
		t.Fatal("terminal Goal collaboration edge still deferred continuation")
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

func TestRoomGoalProgressRequiresConfirmedGoalExecutionAuthority(t *testing.T) {
	service := &Service{goals: &fakeRoomGoalContextProvider{}}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "agent_round_goal_bound_progress",
	}
	if !slot.grantGoalMutationAuthority(roomGoalMutationAuthority{
		SessionKey:        "room:group:test",
		GoalID:            "goal-1",
		ObjectiveRevision: 1,
		RootRoundID:       "root-1",
		Source:            roomGoalAuthorityExplicitRound,
	}) {
		t.Fatal("grant Goal-only authority")
	}
	message := roomGoalToolResultAssistantMessage(
		"tool-workgraph",
		"Bash",
		4,
		1,
	)
	content := message["content"].([]map[string]any)
	content[1]["content"] = `{"outcome":"applied"}`
	stageRoomRuntimeCommandReceipt(slot, nexusmcp.CommandReceipt{
		Domain: command.DomainExecution, Operation: "submit_work",
		Outcome: string(protocol.MutationResultApplied), GoalBound: false,
	})
	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, message)
	if slot.hasGoalToolProgress() {
		t.Fatal("Goal-only Room authority counted an unrelated WorkGraph mutation")
	}

	if !slot.ensureResponsibilityAuthorityState().ConfirmGoalExecution(
		"goal-1",
		1,
		"execution-1",
	) {
		t.Fatal("confirm Goal-bound Execution authority")
	}
	stageRoomRuntimeCommandReceipt(slot, nexusmcp.CommandReceipt{
		Domain: command.DomainExecution, Operation: "submit_work",
		Outcome: string(protocol.MutationResultApplied), GoalBound: true,
	})
	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, message)
	if !slot.hasGoalToolProgress() {
		t.Fatal("confirmed Goal-bound Room WorkGraph mutation was not counted")
	}
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
