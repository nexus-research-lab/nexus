package realtime

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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

func TestRoomGoalMutationAuthorityRequiresExecutionBinding(t *testing.T) {
	authority := roomGoalMutationAuthority{
		SessionKey:        "room:group:conversation-1",
		GoalID:            "goal-room",
		ObjectiveRevision: 1,
		RootRoundID:       "round-1",
		Source:            roomGoalAuthorityExplicitRound,
	}
	if authority.valid() {
		t.Fatal("Goal authority without Execution identity was accepted")
	}
	authority.ExecutionID = "execution-room"
	if !authority.valid() {
		t.Fatal("complete Goal authority was rejected")
	}
}

func TestRoomGoalContinuationRequestRequiresExecutionBinding(t *testing.T) {
	request := ChatRequest{
		SessionKey:            "room:group:conversation-1",
		ConversationID:        "conversation-1",
		GoalContext:           "continue",
		GoalID:                "goal-room",
		GoalObjectiveRevision: 1,
		Internal:              true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}
	if _, _, err := (&Service{}).validateChatRequest(request); err == nil {
		t.Fatal("Goal continuation request without Execution identity was accepted")
	}
	request.ExecutionID = "execution-room"
	if _, _, err := (&Service{}).validateChatRequest(request); err != nil {
		t.Fatalf("complete Goal continuation request rejected: %v", err)
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

func TestRoomRoundContinuationOptionsMarkedHiddenSynthetic(t *testing.T) {
	roundValue := &activeRoomRound{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:  "goal_continuation",
			Metadata: map[string]string{"goal_id": "goal-room"},
		},
	}

	inputOptions := roomRoundInputOptions(roundValue)
	if !inputOptions.HiddenFromUser || !inputOptions.Synthetic || inputOptions.Priority != "internal" {
		t.Fatalf("input options = %#v, want hidden synthetic internal continuation", inputOptions)
	}
	if inputOptions.Purpose != "goal_continuation" || inputOptions.Metadata["goal_id"] != "goal-room" {
		t.Fatalf("input options = %#v, want continuation metadata preserved", inputOptions)
	}

	markerOptions := roomRoundMarkerOptions(roundValue)
	if !markerOptions.HiddenFromUser || !markerOptions.Synthetic {
		t.Fatalf("marker options = %#v, want hidden synthetic round marker", markerOptions)
	}
	if markerOptions.Purpose != "goal_continuation" || markerOptions.Metadata["goal_id"] != "goal-room" {
		t.Fatalf("marker options = %#v, want continuation metadata preserved", markerOptions)
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

func TestShouldBroadcastRoomChatAckForInternalGoalContinuation(t *testing.T) {
	if !shouldBroadcastRoomChatAck(ChatRequest{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}) {
		t.Fatal("internal Room Goal continuation should publish chat_ack for visible execution state")
	}
	if shouldBroadcastRoomChatAck(ChatRequest{Internal: true}) {
		t.Fatal("ordinary internal Room turns should remain hidden from chat_ack")
	}
	if !shouldBroadcastRoomChatAck(ChatRequest{}) {
		t.Fatal("public Room turns should publish chat_ack")
	}
}

func TestBuildRoomGoalCollaborationContextRequiresPublicDelegation(t *testing.T) {
	contextValue := buildRoomGoalCollaborationContext(map[string]string{
		"agent-lead":  "负责人",
		"agent-alpha": "Alpha",
		"agent-beta":  "Beta",
	}, "agent-lead")

	for _, expected := range []string{
		"Visible collaboration is a required part",
		"Lead agent for this continuation: 负责人 (agent_id=agent-lead)",
		"@Alpha (agent_id=agent-alpha)",
		"@Beta (agent_id=agent-beta)",
		"assess task complexity, separable work, and member fit",
		"meaningful independent deliverable rather than ceremonial work",
		"must @ exactly one target",
		"do not independently duplicate that deliverable",
		"coordination, unblocking, integration, and verification",
		"Do not call the Goal update tool in the same turn",
		"Completion requires room-visible collaborator evidence",
	} {
		if !strings.Contains(contextValue, expected) {
			t.Fatalf("collaboration context missing %q:\n%s", expected, contextValue)
		}
	}
	if strings.Contains(contextValue, "@负责人") {
		t.Fatalf("collaboration context should not delegate to lead:\n%s", contextValue)
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

func TestGoalContinuationTargetAgentIDPrefersRoomGoalLead(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			HostAgentID:          "agent-host",
			HostAutoReplyEnabled: false,
		},
	}
	agentNameByID := map[string]string{
		"agent-host": "主持人",
		"agent-lead": "负责人",
	}
	goal := &protocol.Goal{
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalLeadAgentID: "agent-lead",
		},
	}

	targetAgentID := goalContinuationTargetAgentID(contextValue, agentNameByID, goal)

	if targetAgentID != "agent-lead" {
		t.Fatalf("targetAgentID = %q, want metadata lead", targetAgentID)
	}
}

func TestGoalContinuationTargetAgentIDUsesHostWithoutAutoReply(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			HostAgentID:          "agent-host",
			HostAutoReplyEnabled: false,
		},
	}
	agentNameByID := map[string]string{
		"agent-host": "主持人",
		"agent-peer": "成员",
	}

	targetAgentID := goalContinuationTargetAgentID(contextValue, agentNameByID, nil)

	if targetAgentID != "agent-host" {
		t.Fatalf("targetAgentID = %q, want room host even when auto reply is disabled", targetAgentID)
	}
}

type fakeRoomGoalLeadReconciler struct {
	*fakeRoomGoalContextProvider
	current           *protocol.Goal
	assignedGoalID    string
	assignedAgentID   string
	assignedAgentName string
}

func (f *fakeRoomGoalLeadReconciler) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return f.current, nil
}

func (f *fakeRoomGoalLeadReconciler) SetRoomGoalLead(_ context.Context, goalID string, agentID string, agentName string) (*protocol.Goal, error) {
	f.assignedGoalID = goalID
	f.assignedAgentID = agentID
	f.assignedAgentName = agentName
	return f.current, nil
}

func TestReconcileRoomGoalLeadUsesValidRoomHost(t *testing.T) {
	goalProvider := &fakeRoomGoalLeadReconciler{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		current: &protocol.Goal{
			ID:         "goal-room",
			SessionKey: protocol.BuildRoomSharedSessionKey("conversation-1"),
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataRoomGoalLeadAgentID: "agent-removed",
			},
		},
	}
	service := &Service{goals: goalProvider}
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{HostAgentID: "agent-host"},
	}
	err := service.reconcileRoomGoalLead(
		context.Background(),
		protocol.BuildRoomSharedSessionKey("conversation-1"),
		contextValue,
		map[string]string{"agent-host": "Host", "agent-peer": "Peer"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if goalProvider.assignedGoalID != "goal-room" || goalProvider.assignedAgentID != "agent-host" || goalProvider.assignedAgentName != "Host" {
		t.Fatalf("lead assignment = goal:%q agent:%q name:%q", goalProvider.assignedGoalID, goalProvider.assignedAgentID, goalProvider.assignedAgentName)
	}
}

func TestRealtimeServicePostRoundWorkPlansRoomGoalContinuation(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{
		goals: goalProvider,
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
	}
	attachTestRoomGoalAuthority(roundValue, "goal-room")

	service.dispatchPostRoundWork(context.Background(), roundValue)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if goalProvider.planCalls != 1 {
		t.Fatalf("planCalls = %d, want post-round room goal continuation planning", goalProvider.planCalls)
	}
}

func TestRealtimeServiceReleasesSubagentWaitAndPlansRoomGoalContinuation(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{
		goals: goalProvider,
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
		Slots: map[string]*activeRoomSlot{
			"agent-1": {AgentID: "agent-1"},
		},
	}
	roundValue.RunningSubagents.Store(true)
	grantTestRoomGoalAuthority(
		roundValue.Slots["agent-1"],
		roundValue.SessionKey,
		"goal-room",
	)

	service.releaseRoundSubagentWait(roundValue)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if roundValue.RunningSubagents.Load() {
		t.Fatal("RunningSubagents = true, want released after all subagent tasks finish")
	}
	if goalProvider.planCalls != 1 {
		t.Fatalf("planCalls = %d, want post-subagent room goal continuation planning", goalProvider.planCalls)
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
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-goal-room",
				},
			},
			RoundID: "goal_continuation_1",
		},
	}
	goalProvider.onPlan = func() {
		runtimeManager.StartRound("room:group:conversation-1", "queued-user-round", nil)
	}
	service := &Service{
		goals:   goalProvider,
		runtime: runtimeManager,
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
	}
	attachTestRoomGoalAuthority(roundValue, "goal-room")

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
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-goal-room",
				},
			},
			RoundID: "goal_continuation_1",
		},
	}
	service := &Service{
		goals: goalProvider,
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
	}
	attachTestRoomGoalAuthority(roundValue, "goal-room")

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

func TestShouldDeferGoalContinuationWhileCollaboratorSlotIsActive(t *testing.T) {
	const conversationID = "conversation-active-collaborator"
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	peerSlot := withRoomSlotStatus(&activeRoomSlot{AgentID: "agent-peer"}, "running")
	service := &Service{
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"peer-round": {
				SessionKey:     sessionKey,
				ConversationID: conversationID,
				RoundID:        "round-peer",
				Slots:          map[string]*activeRoomSlot{"peer": peerSlot},
			},
		}),
	}
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: conversationID},
	}

	if !service.shouldDeferGoalContinuationForTargetState(context.Background(), sessionKey, contextValue) {
		t.Fatal("continuation should defer while a collaborator slot is active")
	}
	peerSlot.setStatus("finished")
	if service.shouldDeferGoalContinuationForTargetState(context.Background(), sessionKey, contextValue) {
		t.Fatal("continuation should not defer on target state after collaborator slot becomes terminal")
	}
}

// Goal 续接进度测试。

func TestRecordGoalContinuationProgressForRoomSlotSuppressesEmptyContinuation(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalContinuationProgressForSlot(context.Background(), slot, roundValue, exec.RoundExecutionResult{}, nil)

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || progress[0] {
		t.Fatalf("progress = %#v, want one false continuation progress", progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotDefersWhileSubagentRuns(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	slot.setSubagentTasks(map[string]struct{}{"task-1": {}})
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalContinuationProgressForSlot(context.Background(), slot, roundValue, exec.RoundExecutionResult{}, nil)

	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want running subagent to defer empty continuation progress", progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsFailure(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	roundValue := &activeRoomRound{
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
}

func TestRecordGoalContinuationProgressForRoomSlotCountsToolProgress(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	service.recordGoalContinuationProgressForSlot(context.Background(), slot, roundValue, exec.RoundExecutionResult{}, nil)

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || !progress[0] {
		t.Fatalf("progress = %#v, want one true continuation progress", progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsCompletionToolMiss(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalContinuationProgressForSlot(
		context.Background(),
		slot,
		roundValue,
		exec.RoundExecutionResult{},
		roomGoalCompletionToolMissAssistantMessage(),
	)

	misses := goalProvider.recordedCompletionMisses()
	if len(misses) != 1 || !strings.Contains(misses[0], "mcp__nexus_goal__update_goal") {
		t.Fatalf("completion misses = %#v, want one missing update_goal record", misses)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want completion miss path instead of empty progress", progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsUserActivity(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-user",
	}
	grantTestRoomGoalAuthority(slot, "room:group:test", "goal-1")
	roundValue := &activeRoomRound{}

	service.recordGoalContinuationProgressForSlot(context.Background(), slot, roundValue, exec.RoundExecutionResult{}, nil)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.activities) != 1 || goalProvider.activities[0] != "round-user" {
		t.Fatalf("activities = %#v, want explicit room goal activity", goalProvider.activities)
	}
	if len(goalProvider.progress) != 0 {
		t.Fatalf("progress = %#v, want no continuation progress for user room round", goalProvider.progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsCollaborationEvidence(t *testing.T) {
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
		roomGoalTextAssistantMessage("peer-reply", "我完成了调研。"),
	)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.collabEvidence) != 1 || goalProvider.collabEvidence[0] != "room_mention_1:agent-peer" {
		t.Fatalf("collaboration evidence = %#v, want peer evidence", goalProvider.collabEvidence)
	}
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
