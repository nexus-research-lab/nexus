package realtime

import (
	"context"
	"errors"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
)

func TestRoomRuntimeMCPContextClonesStructuredWorkBinding(t *testing.T) {
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	var captured *protocol.ExecutionWorkBinding
	service := &Service{
		executionMCPServers: func(
			_ context.Context,
			value runtimectx.ExecutionToolContext,
		) map[string]sdkmcp.ServerConfig {
			captured = value.WorkBinding
			if value.WorkBinding != nil {
				value.WorkBinding.AssignmentID = "assignment-mutated"
			}
			return nil
		},
	}
	execution := &slotExecution{
		service: service,
		ctx:     context.Background(),
		round: &activeRoomRound{
			SessionKey:         "room:group:conversation-1",
			RoomID:             "room-1",
			ConversationID:     "conversation-1",
			RootRoundID:        "root-round-1",
			CoordinatorAgentID: "agent-lead",
		},
		slot: &activeRoomSlot{
			AgentID:           "agent-member",
			AgentRoundID:      "agent-round-1",
			RuntimeSessionKey: "runtime-session-1",
			WorkBinding:       binding,
		},
		agent: &protocol.Agent{
			AgentID:     "agent-member",
			OwnerUserID: "owner-1",
		},
	}

	execution.runtimeMCPServers("")

	if captured == nil || captured == binding {
		t.Fatalf("captured WorkBinding = %#v", captured)
	}
	if binding.AssignmentID != "assignment-1" {
		t.Fatal("Execution MCP builder mutated the Room slot WorkBinding")
	}
}

func TestRoomRuntimeSharesDynamicSelfWorkBindingWithMCPAndGraphActor(t *testing.T) {
	var state *runtimectx.WorkBindingState
	service := &Service{
		executionMCPServers: func(
			_ context.Context,
			value runtimectx.ExecutionToolContext,
		) map[string]sdkmcp.ServerConfig {
			state = value.WorkBindingState
			return nil
		},
	}
	execution := &slotExecution{
		service: service,
		ctx:     context.Background(),
		round: &activeRoomRound{
			OwnerUserID: "owner-1", SessionKey: "room:group:conversation-1",
			RoomID: "room-1", ConversationID: "conversation-1",
			RootRoundID: "root-round-1", ExecutionID: "execution-1",
			CoordinatorAgentID: "agent-lead",
		},
		slot: &activeRoomSlot{
			AgentID: "agent-lead", AgentRoundID: "agent-round-1",
			RuntimeSessionKey: "runtime-session-1",
		},
		agent: &protocol.Agent{AgentID: "agent-lead", OwnerUserID: "owner-1"},
	}

	execution.runtimeMCPServers("")
	if state == nil || state != execution.ensureWorkBindingState() {
		t.Fatalf("Execution MCP WorkBindingState = %#v", state)
	}
	if actor := execution.orchestrationActor(); actor.WorkBinding != nil {
		t.Fatalf("ordinary Room coordinator actor = %+v", actor)
	}
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	if !state.Bind(binding) {
		t.Fatal("bind Room self WorkBinding")
	}
	actor := execution.orchestrationActor()
	if actor.WorkBinding == nil || actor.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatalf("runtime graph actor did not observe WorkBinding: %+v", actor)
	}
}

type roomAttemptTerminalizerFake struct {
	calls  []orchestrationsvc.RoomAttemptTerminalInput
	actors []orchestrationsvc.ActorContext
	ctxErr error
	err    error
}

func (f *roomAttemptTerminalizerFake) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (f *roomAttemptTerminalizerFake) FinishRoomAttempt(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
	input orchestrationsvc.RoomAttemptTerminalInput,
) error {
	f.ctxErr = ctx.Err()
	f.actors = append(f.actors, actor)
	f.calls = append(f.calls, input)
	return f.err
}

func TestStructuredRoomSlotCompletionSettlesRootAttemptWithoutSemanticSubmission(t *testing.T) {
	binding := testRoomExecutionWorkBinding()
	terminalizer := &roomAttemptTerminalizerFake{}
	service := &Service{
		executionContext: terminalizer,
		permission:       permissionctx.NewContext(),
	}
	roundValue := &activeRoomRound{
		SessionKey:         "room:group:conversation-1",
		RoomID:             "room-1",
		ConversationID:     "conversation-1",
		CoordinatorAgentID: "agent-lead",
		RootRoundID:        "root-round-1",
		OwnerUserID:        "owner-1",
	}
	slot := &activeRoomSlot{
		RoomSessionID:     "room-session-1",
		AgentID:           "agent-member",
		AgentRoundID:      "agent-round-1",
		RuntimeSessionKey: "runtime-session-1",
		WorkBinding:       binding,
	}
	slot.setStatus("running")
	slot.setDeliveryMetadata(
		protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePrivate},
		"",
		"",
	)
	authorizeRoomExecutionTestSlot(service, roundValue, slot)
	execution := &slotExecution{
		service: service,
		ctx:     context.Background(),
		round:   roundValue,
		slot:    slot,
		mapper: roomdomain.NewSlotMessageMapper(
			roundValue.SessionKey,
			roundValue.RoomID,
			roundValue.ConversationID,
			slot.AgentID,
			"message-1",
			roundValue.RootRoundID,
			slot.AgentRoundID,
			t.TempDir(),
		),
	}

	if err := execution.complete(exec.RoundExecutionResult{
		TerminalStatus: "finished",
		ResultSubtype:  "success",
	}); err != nil {
		t.Fatal(err)
	}
	assertRoomAttemptTerminalCall(
		t,
		terminalizer,
		protocol.WorkAttemptStatusSucceeded,
		"",
	)
}

func TestStructuredRoomSlotFailureAndCancellationSettleRootAttempt(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		slotStatus string
		reason     string
		wantStatus protocol.WorkAttemptStatus
	}{
		{
			name:       "error",
			slotStatus: "error",
			reason:     "runtime failed",
			wantStatus: protocol.WorkAttemptStatusFailed,
		},
		{
			name:       "interrupted",
			slotStatus: "interrupted",
			reason:     "user stopped",
			wantStatus: protocol.WorkAttemptStatusInterrupted,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			terminalizer := &roomAttemptTerminalizerFake{}
			service := &Service{executionContext: terminalizer}
			roundValue := &activeRoomRound{
				SessionKey:         "room:group:conversation-1",
				RoomID:             "room-1",
				ConversationID:     "conversation-1",
				CoordinatorAgentID: "agent-lead",
				RootRoundID:        "root-round-1",
				OwnerUserID:        "owner-1",
			}
			slot := &activeRoomSlot{
				RoomSessionID:     "room-session-1",
				AgentID:           "agent-member",
				AgentRoundID:      "agent-round-1",
				RuntimeSessionKey: "runtime-session-1",
				WorkBinding:       testRoomExecutionWorkBinding(),
			}
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			if err := service.finishBoundRoomAttempt(
				cancelled,
				roundValue,
				slot,
				testCase.slotStatus,
				testCase.reason,
			); err != nil {
				t.Fatal(err)
			}
			assertRoomAttemptTerminalCall(
				t,
				terminalizer,
				testCase.wantStatus,
				testCase.reason,
			)
			if terminalizer.ctxErr != nil {
				t.Fatalf("terminal settlement inherited cancelled slot context: %v", terminalizer.ctxErr)
			}
		})
	}
}

func TestDynamicRoomSelfBindingFailureSettlesRootAttempt(t *testing.T) {
	terminalizer := &roomAttemptTerminalizerFake{}
	service := &Service{executionContext: terminalizer}
	roundValue := &activeRoomRound{
		SessionKey: "room:group:conversation-1", RoomID: "room-1",
		ConversationID: "conversation-1", CoordinatorAgentID: "agent-lead",
		RootRoundID: "root-round-1", OwnerUserID: "owner-1",
	}
	slot := &activeRoomSlot{
		RoomSessionID: "room-session-1", AgentID: "agent-lead",
		AgentRoundID: "agent-round-1", RuntimeSessionKey: "runtime-session-1",
	}
	binding := testRoomExecutionWorkBinding()
	binding.DispatchID = ""
	if !slot.ensureWorkBindingState().Bind(binding) {
		t.Fatal("bind dynamic Room self WorkBinding")
	}

	if err := service.finishBoundRoomAttempt(
		context.Background(),
		roundValue,
		slot,
		"error",
		"runtime failed before submit_work",
	); err != nil {
		t.Fatal(err)
	}
	assertRoomAttemptTerminalCall(
		t,
		terminalizer,
		protocol.WorkAttemptStatusFailed,
		"runtime failed before submit_work",
	)
	if terminalizer.calls[0].Binding.DispatchID != "" {
		t.Fatalf("self WorkBinding terminal call = %#v", terminalizer.calls[0].Binding)
	}
}

func TestHandleStructuredRoomSlotFailureClosesBoundRootAttempt(t *testing.T) {
	terminalizer := &roomAttemptTerminalizerFake{}
	service := &Service{
		executionContext: terminalizer,
		permission:       permissionctx.NewContext(),
		roomHistory:      workspacestore.NewRoomHistoryStore(t.TempDir()),
	}
	roundValue := &activeRoomRound{
		SessionKey:         "room:group:conversation-1",
		RoomID:             "room-1",
		ConversationID:     "conversation-1",
		CoordinatorAgentID: "agent-lead",
		RootRoundID:        "root-round-1",
		OwnerUserID:        "owner-1",
	}
	slot := &activeRoomSlot{
		OwnerUserID:       "owner-1",
		RoomSessionID:     "room-session-1",
		AgentID:           "agent-member",
		AgentRoundID:      "agent-round-1",
		RuntimeSessionKey: "runtime-session-1",
		WorkBinding:       testRoomExecutionWorkBinding(),
	}
	slot.setStatus("running")
	slot.setDeliveryMetadata(
		protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePrivate},
		"",
		"",
	)
	authorizeRoomExecutionTestSlot(service, roundValue, slot)

	service.handleSlotFailure(
		context.Background(),
		roundValue,
		slot,
		nil,
		exec.RoundExecutionResult{},
		errors.New("runtime failed"),
	)

	assertRoomAttemptTerminalCall(
		t,
		terminalizer,
		protocol.WorkAttemptStatusFailed,
		"runtime failed",
	)
	if slot.getStatus() != "error" {
		t.Fatalf("slot status = %q", slot.getStatus())
	}
}

func authorizeRoomExecutionTestSlot(
	service *Service,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:                     roundValue.RoomID,
			RoomType:               protocol.RoomTypeGroup,
			PrivateMessagesEnabled: true,
			AuthorityEpoch:         1,
		},
		Conversation: protocol.ConversationRecord{
			ID:     roundValue.ConversationID,
			RoomID: roundValue.RoomID,
		},
		Members: []protocol.MemberRecord{{
			MemberType:    protocol.MemberTypeAgent,
			MemberAgentID: slot.AgentID,
		}},
	}
	service.rooms = &authorityFenceRoomStore{contextValue: contextValue}
	roundValue.Context = cloneAuthorityFenceContext(contextValue)
	roundValue.AuthorityEpoch = contextValue.Room.AuthorityEpoch
}

func TestStructuredRoomSlotTerminalizerFailureIsReturnedToCompletion(t *testing.T) {
	terminalizer := &roomAttemptTerminalizerFake{err: errors.New("database unavailable")}
	service := &Service{executionContext: terminalizer}
	err := service.finishBoundRoomAttempt(
		context.Background(),
		&activeRoomRound{
			SessionKey:     "room:group:conversation-1",
			RoomID:         "room-1",
			ConversationID: "conversation-1",
			RootRoundID:    "root-round-1",
			OwnerUserID:    "owner-1",
		},
		&activeRoomSlot{
			AgentID:           "agent-member",
			AgentRoundID:      "agent-round-1",
			RuntimeSessionKey: "runtime-session-1",
			WorkBinding:       testRoomExecutionWorkBinding(),
		},
		"finished",
		"",
	)
	if err == nil || err.Error() != "database unavailable" {
		t.Fatalf("finishBoundRoomAttempt error = %v", err)
	}
}

func TestGenericRoomQueueControlCannotEraseResponsibility(t *testing.T) {
	conversation := protocol.InputQueueItem{ID: "conversation-1"}
	if err := rejectGenericControlForBoundQueueItem(conversation, "delete"); err != nil {
		t.Fatalf("ordinary conversation queue control error = %v", err)
	}

	for _, item := range []protocol.InputQueueItem{
		{
			ID:          "work-1",
			WorkBinding: testRoomExecutionWorkBinding(),
		},
		{
			ID: "review-1",
			ReviewBinding: &protocol.ExecutionReviewBinding{
				ExecutionID:      "execution-1",
				PlanID:           "plan-1",
				WorkItemID:       "work-1",
				SpecID:           "spec-1",
				AssignmentID:     "assignment-1",
				SubmissionID:     "submission-1",
				ReviewDispatchID: "review-dispatch-1",
				TargetAgentID:    "agent-lead",
			},
		},
	} {
		for _, action := range []string{"delete", "guide", "reorder"} {
			err := rejectGenericControlForBoundQueueItem(item, action)
			if !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
				t.Fatalf("%s %s error = %v, want capability envelope error", action, item.ID, err)
			}
		}
	}
}

func testRoomExecutionWorkBinding() *protocol.ExecutionWorkBinding {
	return &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
}

func assertRoomAttemptTerminalCall(
	t *testing.T,
	terminalizer *roomAttemptTerminalizerFake,
	wantStatus protocol.WorkAttemptStatus,
	wantReason string,
) {
	t.Helper()
	if len(terminalizer.calls) != 1 || len(terminalizer.actors) != 1 {
		t.Fatalf("terminal calls = %#v actors = %#v", terminalizer.calls, terminalizer.actors)
	}
	call := terminalizer.calls[0]
	actor := terminalizer.actors[0]
	if call.Status != wantStatus ||
		call.FailureReason != wantReason ||
		call.Binding.AssignmentID != "assignment-1" ||
		call.RuntimeSessionKey != "runtime-session-1" ||
		actor.WorkBinding == nil ||
		actor.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatalf("terminal call = %#v actor = %#v", call, actor)
	}
}
