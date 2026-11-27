// execution_test.go 覆盖 Room 执行派发、取消、评审派发、WorkBinding、权威校验与恢复测试。
package realtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type managedExecutionAdmissionFake struct {
	actor   orchestrationsvc.ActorContext
	binding *protocol.ExecutionWorkBinding
	err     error
}

func (f *managedExecutionAdmissionFake) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (f *managedExecutionAdmissionFake) AuthorizeRoomRuntimeTarget(
	_ context.Context,
	actor orchestrationsvc.ActorContext,
	binding *protocol.ExecutionWorkBinding,
) error {
	f.actor = actor
	f.binding = binding.Clone()
	return f.err
}

func TestStaleStructuredWorkWakeIsTerminalNotRetriedAsConversation(t *testing.T) {
	binding := testRoomExecutionWorkBinding()
	parent := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RootRoundID:    "root-1",
		OwnerUserID:    "owner-1",
		Context:        &protocol.ConversationContextAggregate{},
	}
	wake := publicMentionWake{
		HandoffID:     "execution_dispatch_dispatch-1",
		TargetAgentID: "agent-worker",
		WorkBinding:   binding,
	}

	permanent := withConstructorDefaults(t, &Service{Host: runtimehost.Host{ExecutionContext: &managedExecutionAdmissionFake{
		err: &orchestrationsvc.DomainError{
			Code:    orchestrationsvc.ErrorCodeWorkBindingMismatch,
			Message: "the bound Attempt was superseded",
		},
	}},
	})
	if err := permanent.startPublicMentionRoundLocked(
		context.Background(),
		parent,
		[]publicMentionWake{wake},
	); err != nil {
		t.Fatalf("stale structured wake should be consumed as terminal: %v", err)
	}

	transientErr := errors.New("database temporarily unavailable")
	transient := withConstructorDefaults(t, &Service{Host: runtimehost.Host{ExecutionContext: &managedExecutionAdmissionFake{err: transientErr}}})
	if err := transient.startPublicMentionRoundLocked(
		context.Background(),
		parent,
		[]publicMentionWake{wake},
	); !errors.Is(err, transientErr) {
		t.Fatalf("transient admission error = %v, want retryable error", err)
	}
}

func TestStructuredHandoffRecoveryRequiresExactQueueCapability(t *testing.T) {
	binding := testRoomExecutionWorkBinding()
	handoff := workspacestore.RoomPublicHandoff{
		HandoffID:       "execution_dispatch_dispatch-1",
		OwnerUserID:     "owner-1",
		ConversationID:  "conversation-1",
		RoomID:          "room-1",
		RootRoundID:     "execution_dispatch_dispatch-1",
		SourceMessageID: "execution_dispatch_dispatch-1",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-worker",
		Content:         "deliver the evidence",
		WorkBinding:     binding,
	}
	item := protocol.InputQueueItem{
		ID:              handoff.HandoffID,
		Scope:           protocol.InputQueueScopeRoom,
		RoomID:          handoff.RoomID,
		ConversationID:  handoff.ConversationID,
		AgentID:         handoff.TargetAgentID,
		SourceAgentID:   handoff.SourceAgentID,
		SourceMessageID: handoff.SourceMessageID,
		HandoffID:       handoff.HandoffID,
		TargetAgentIDs:  []string{handoff.TargetAgentID},
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		Content:         handoff.Content,
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		OwnerUserID:     handoff.OwnerUserID,
		RootRoundID:     handoff.RootRoundID,
		WorkBinding:     binding,
	}
	if !inputQueueItemMatchesDurableHandoff(item, handoff) {
		t.Fatal("exact structured queue item did not match durable handoff")
	}
	item.WorkBinding = nil
	if inputQueueItemMatchesDurableHandoff(item, handoff) {
		t.Fatal("ordinary queue item suppressed structured handoff recovery")
	}
	item.WorkBinding = binding
	item.DeliveryPolicy = protocol.ChatDeliveryPolicyGuide
	if inputQueueItemMatchesDurableHandoff(item, handoff) {
		t.Fatal("guided queue item suppressed structured handoff recovery")
	}
}

func TestExecutionDispatchInstructionCarriesCompleteWorkBinding(t *testing.T) {
	delivery := orchestrationsvc.ExecutionDispatchDelivery{
		OwnerUserID:    "owner-1",
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-lead",
		TargetAgentID:  "agent-worker",
		Kind:           protocol.ExecutionDispatchRoomDirected,
		Instruction:    "Deliver the evidence set.",
		WorkContract: orchestrationsvc.ExecutionDispatchWorkContract{
			InputRefs: []string{"artifact://accepted-upstream"},
			OutputScopes: []protocol.WorkOutputScope{{
				Scope: "file:reports/evidence.md",
				Mode:  protocol.WorkOutputScopeExclusive,
			}},
			AcceptedDependencies: []orchestrationsvc.ExecutionAcceptedDependency{{
				WorkItemID:    "work-upstream",
				LogicalKey:    "W0",
				SpecID:        "spec-upstream",
				Kind:          protocol.WorkDependencyHard,
				SubmissionID:  "submission-upstream",
				ResultSummary: "Verified upstream evidence",
				ResultRefs:    []string{"artifact://upstream-result"},
				AcceptanceID:  "acceptance-upstream",
			}},
		},
		DispatchDedupeKey: "dispatch:work-1:agent-worker",
		Binding: protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			AttemptID:    "attempt-1",
			DispatchID:   "dispatch-1",
		},
	}
	if err := validateExecutionDispatchDelivery(delivery); err != nil {
		t.Fatal(err)
	}
	submissionSchema := `{"type":"object","required":["result_summary"],"properties":{"result_summary":{"type":"string"},"evidence":{"type":"array","items":{"type":"string"}}}}`
	instruction := renderExecutionDispatchInstruction(delivery, submissionSchema)
	for _, expected := range []string{
		"execution_id: execution-1",
		"plan_id: plan-1",
		"work_item_id: work-1",
		"spec_id: spec-1",
		"assignment_id: assignment-1",
		"attempt_id: attempt-1",
		"dispatch_id: dispatch-1",
		`- "artifact://accepted-upstream"`,
		`- mode=exclusive scope="file:reports/evidence.md"`,
		`work_item_id=work-upstream logical_key="W0" spec_id=spec-upstream kind=hard submission_id=submission-upstream acceptance_id=acceptance-upstream`,
		`result_summary: "Verified upstream evidence"`,
		`- "artifact://upstream-result"`,
		"Deliver the evidence set.",
		"operation=submit_work",
		"request_id at the tool top level",
		"[submission_contract]",
		submissionSchema,
	} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("structured instruction missing %q:\n%s", expected, instruction)
		}
	}

	incomplete := delivery
	incomplete.Binding.AttemptID = ""
	if err := validateExecutionDispatchDelivery(incomplete); err == nil {
		t.Fatal("delivery without Attempt binding was accepted")
	}
}

func TestExecutionDispatchHandoffIsDurableAndIdempotentBeforeSQLAck(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	store := workspacestore.NewRoomPublicHandoffStore(stateRoot)
	service := &Service{publicHandoffs: store}
	delivery := orchestrationsvc.ExecutionDispatchDelivery{
		OwnerUserID:    "owner-1",
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-lead",
		TargetAgentID:  "agent-worker",
		Kind:           protocol.ExecutionDispatchRoomDirected,
		Instruction:    "Deliver the evidence set.",
		Binding: protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			AttemptID:    "attempt-1",
			DispatchID:   "dispatch-1",
		},
	}
	handoffID := "execution_dispatch_dispatch-1"
	accepted, _, err := service.ensureExecutionDispatchHandoff(
		delivery,
		handoffID,
		renderExecutionDispatchInstruction(delivery, ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	if accepted {
		t.Fatal("new durable handoff must still be claimed by the Room delivery path")
	}
	handoff, ok, err := store.Get("owner-1", "conversation-1", handoffID)
	if err != nil || !ok {
		t.Fatalf("durable handoff = %+v, ok=%t, err=%v", handoff, ok, err)
	}
	if handoff.Status != "source_finished" ||
		!executionWorkBindingEqual(handoff.WorkBinding, &delivery.Binding) {
		t.Fatalf("durable handoff = %+v", handoff)
	}
	if _, claimed, claimErr := store.Claim("owner-1", "conversation-1", handoffID); claimErr != nil || !claimed {
		t.Fatalf("claim = %t, err=%v", claimed, claimErr)
	}
	if err = store.MarkStarted("owner-1", "conversation-1", handoffID, "round-1"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending("owner-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 ||
		!executionWorkBindingEqual(pending[0].WorkBinding, &delivery.Binding) {
		t.Fatalf("structured started handoff must remain recoverable: %+v", pending)
	}
	accepted, receipt, err := service.ensureExecutionDispatchHandoff(
		delivery,
		handoffID,
		renderExecutionDispatchInstruction(delivery, ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !accepted || receipt.HandoffID != handoffID {
		t.Fatalf("repeat acceptance = %t, receipt=%+v", accepted, receipt)
	}
}

func TestExecutionCancellationInterruptsExactOldRoomBindingOnly(t *testing.T) {
	service := &Service{
		rounds: newRoomRoundRegistry(),
		Host:   runtimehost.Host{Runtime: runtimectx.NewManager(), Permission: permissionctx.NewContext()},
	}
	oldBinding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-old",
		PlanID:       "plan-old",
		WorkItemID:   "work-old",
		SpecID:       "spec-old",
		AssignmentID: "assignment-old",
		AttemptID:    "attempt-old",
		DispatchID:   "dispatch-old",
	}
	successorBinding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-successor",
		PlanID:       "plan-successor",
		WorkItemID:   "work-successor",
		SpecID:       "spec-successor",
		AssignmentID: "assignment-successor",
		AttemptID:    "attempt-successor",
		DispatchID:   "dispatch-successor",
	}
	oldSlot := &activeRoomSlot{
		AgentID:           "agent-worker",
		AgentRoundID:      "agent-round-old",
		RuntimeSessionKey: "agent:agent-worker:ws:group:conversation-1",
		WorkBinding:       oldBinding,
	}
	successorSlot := &activeRoomSlot{
		AgentID:           "agent-worker",
		AgentRoundID:      "agent-round-successor",
		RuntimeSessionKey: oldSlot.RuntimeSessionKey,
		WorkBinding:       successorBinding,
	}
	oldSlot.setStatus("running")
	successorSlot.setStatus("running")
	oldSlot.doneChannel()
	successorSlot.doneChannel()
	oldCancelled := 0
	successorCancelled := 0
	oldCancel := func() {
		oldCancelled++
		oldSlot.setStatus("cancelled")
		oldSlot.closeDone()
		service.Runtime.MarkRoundFinished(
			oldSlot.RuntimeSessionKey,
			oldSlot.AgentRoundID,
		)
	}
	successorCancel := func() {
		successorCancelled++
		successorSlot.setStatus("cancelled")
		successorSlot.closeDone()
		service.Runtime.MarkRoundFinished(
			successorSlot.RuntimeSessionKey,
			successorSlot.AgentRoundID,
		)
	}
	oldSlot.setCancel(oldCancel)
	successorSlot.setCancel(successorCancel)
	if err := service.Runtime.StartRound(
		context.Background(),
		oldSlot.RuntimeSessionKey,
		oldSlot.AgentRoundID,
		oldCancel,
	); err != nil {
		t.Fatalf("failed to register old runtime round: %v", err)
	}
	if err := service.Runtime.StartRound(
		context.Background(),
		successorSlot.RuntimeSessionKey,
		successorSlot.AgentRoundID,
		successorCancel,
	); err != nil {
		t.Fatalf("failed to register successor runtime round: %v", err)
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RoundID:        "round-active",
		RootRoundID:    "round-active",
		Slots: map[string]*activeRoomSlot{
			"old":       oldSlot,
			"successor": successorSlot,
		},
	}
	service.rounds.register(roundValue)

	receipt, err := service.DeliverExecutionCancellation(
		context.Background(),
		orchestrationsvc.ExecutionCancellationDelivery{
			Binding: protocol.ExecutionCancellationBinding{
				ExecutionID:       oldBinding.ExecutionID,
				PlanID:            oldBinding.PlanID,
				WorkItemID:        oldBinding.WorkItemID,
				SpecID:            oldBinding.SpecID,
				AssignmentID:      oldBinding.AssignmentID,
				AttemptID:         "attempt-child-or-root",
				RuntimeAttemptID:  oldBinding.AttemptID,
				DispatchID:        oldBinding.DispatchID,
				TargetKind:        protocol.ExecutionCancellationTargetRoomSlot,
				TargetAgentID:     oldSlot.AgentID,
				ScopeSessionKey:   roundValue.SessionKey,
				RoomID:            roundValue.RoomID,
				ConversationID:    roundValue.ConversationID,
				RuntimeSessionKey: oldSlot.RuntimeSessionKey,
				RuntimeRoundID:    oldSlot.AgentRoundID,
				AgentRoundID:      oldSlot.AgentRoundID,
			},
			Reason: "old Execution superseded",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != protocol.ExecutionCancellationOutcomeLocalRoundCancelled ||
		receipt.LimitationCode != "provider_interrupt_unsafe_shared_session" ||
		oldCancelled != 1 ||
		successorCancelled != 0 ||
		successorSlot.isTerminal() {
		t.Fatalf(
			"receipt=%+v oldCancelled=%d successorCancelled=%d successorStatus=%s",
			receipt,
			oldCancelled,
			successorCancelled,
			successorSlot.getStatus(),
		)
	}

	staleDelivery := orchestrationsvc.ExecutionCancellationDelivery{
		Binding: protocol.ExecutionCancellationBinding{
			ExecutionID:       oldBinding.ExecutionID,
			PlanID:            oldBinding.PlanID,
			WorkItemID:        oldBinding.WorkItemID,
			SpecID:            oldBinding.SpecID,
			AssignmentID:      oldBinding.AssignmentID,
			AttemptID:         "attempt-old",
			RuntimeAttemptID:  oldBinding.AttemptID,
			DispatchID:        oldBinding.DispatchID,
			TargetKind:        protocol.ExecutionCancellationTargetRoomSlot,
			TargetAgentID:     successorSlot.AgentID,
			ScopeSessionKey:   roundValue.SessionKey,
			RoomID:            roundValue.RoomID,
			ConversationID:    roundValue.ConversationID,
			RuntimeSessionKey: successorSlot.RuntimeSessionKey,
			RuntimeRoundID:    successorSlot.AgentRoundID,
			AgentRoundID:      successorSlot.AgentRoundID,
		},
	}
	receipt, err = service.DeliverExecutionCancellation(
		context.Background(),
		staleDelivery,
	)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != protocol.ExecutionCancellationOutcomeStaleTarget ||
		successorCancelled != 0 {
		t.Fatalf(
			"stale receipt=%+v successorCancelled=%d",
			receipt,
			successorCancelled,
		)
	}
	service.Runtime.MarkRoundFinished(
		successorSlot.RuntimeSessionKey,
		successorSlot.AgentRoundID,
	)
}

type roomExecutionGoalAuthorityProvider struct {
	binding      orchestrationsvc.RuntimeGoalBinding
	resolution   protocol.GoalExecutionBindingResolution
	resolverErr  error
	resolveCalls int
	resolvedGoal protocol.Goal
}

func (p *roomExecutionGoalAuthorityProvider) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (p *roomExecutionGoalAuthorityProvider) RuntimeGoalBinding(
	context.Context,
	orchestrationsvc.ActorContext,
) (orchestrationsvc.RuntimeGoalBinding, error) {
	return p.binding, nil
}

func (p *roomExecutionGoalAuthorityProvider) ResolveGoalExecutionBinding(
	_ context.Context,
	goal protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	p.resolveCalls++
	p.resolvedGoal = goal
	return p.resolution, p.resolverErr
}

func TestRoomWorkAndReviewGoalAuthorityRequiresConfirmedCentralBinding(t *testing.T) {
	states := []struct {
		name        string
		state       protocol.GoalExecutionBindingState
		wantGranted bool
		wantErr     error
	}{
		{name: "standalone remains unbound", state: protocol.GoalExecutionBindingStateStandalone},
		{name: "reserved remains unbound", state: protocol.GoalExecutionBindingStateReserved},
		{name: "pending fails closed", state: protocol.GoalExecutionBindingStatePending, wantErr: goalsvc.ErrGoalExecutionBindingConflict},
		{name: "conflict fails closed", state: protocol.GoalExecutionBindingStateConflict, wantErr: goalsvc.ErrGoalExecutionBindingConflict},
		// The central resolver also emits Confirmed for exact legacy bilateral
		// records; Room deliberately consumes the classification, not provenance.
		{name: "confirmed including legacy classification grants", state: protocol.GoalExecutionBindingStateConfirmed, wantGranted: true},
	}
	actors := []struct {
		name  string
		actor orchestrationsvc.ActorContext
	}{
		{
			name: "work",
			actor: orchestrationsvc.ActorContext{WorkBinding: &protocol.ExecutionWorkBinding{
				ExecutionID: "execution-1",
			}},
		},
		{
			name: "review",
			actor: orchestrationsvc.ActorContext{ReviewBinding: &protocol.ExecutionReviewBinding{
				ExecutionID: "execution-1",
			}},
		},
	}

	for _, actorCase := range actors {
		for _, stateCase := range states {
			t.Run(actorCase.name+"/"+stateCase.name, func(t *testing.T) {
				provider := &roomExecutionGoalAuthorityProvider{
					binding: orchestrationsvc.RuntimeGoalBinding{
						ExecutionID:           "execution-1",
						SessionKey:            "room:group:conversation-1",
						GoalID:                "goal-1",
						GoalObjectiveRevision: 2,
					},
					resolution: protocol.GoalExecutionBindingResolution{
						State:       stateCase.state,
						ExecutionID: "execution-1",
					},
				}
				service := &Service{
					Host: runtimehost.Host{ExecutionContext: provider},
					goals: &fakeRoomGoalContextProvider{
						runtimeContexts: map[string]string{
							"room:group:conversation-1": "authoritative goal context",
						},
						runtimeGoals: map[string]*protocol.Goal{
							"room:group:conversation-1": {
								ID:         "goal-1",
								SessionKey: "room:group:conversation-1",
								Metadata: map[string]any{
									protocol.GoalMetadataObjectiveRevision: int64(2),
								},
							},
						},
					},
				}

				goalContext, authority, granted, err := service.resolveExecutionGoalMutationAuthority(
					context.Background(),
					actorCase.actor,
					"root-round-1",
				)
				if !errors.Is(err, stateCase.wantErr) {
					t.Fatalf("error = %v, want %v", err, stateCase.wantErr)
				}
				if granted != stateCase.wantGranted {
					t.Fatalf("granted = %v, want %v", granted, stateCase.wantGranted)
				}
				if provider.resolveCalls != 1 || provider.resolvedGoal.ID != "goal-1" || provider.resolvedGoal.ObjectiveRevision() != 2 {
					t.Fatalf("resolver calls = %d, goal = %#v", provider.resolveCalls, provider.resolvedGoal)
				}
				if !stateCase.wantGranted {
					if goalContext != "" || authority.valid() {
						t.Fatalf("unbound result context=%q authority=%+v", goalContext, authority)
					}
					return
				}
				if goalContext != "authoritative goal context" ||
					authority.SessionKey != "room:group:conversation-1" ||
					authority.GoalID != "goal-1" ||
					authority.ObjectiveRevision != 2 ||
					authority.ExecutionID != "execution-1" ||
					authority.RootRoundID != "root-round-1" ||
					authority.Source != roomGoalAuthorityExecutionBinding {
					t.Fatalf("confirmed result context=%q authority=%+v", goalContext, authority)
				}
			})
		}
	}
}

func TestReviewDispatchReusesInjectedSchemaWithoutGrantingAuthority(t *testing.T) {
	delivery := orchestrationsvc.ExecutionReviewDispatchDelivery{
		Instruction: "Review the cited evidence against each criterion.",
		Binding: protocol.ExecutionReviewBinding{
			ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1", SpecID: "spec-1",
			AssignmentID: "assignment-1", SubmissionID: "submission-1", ReviewDispatchID: "review-1", TargetAgentID: "reviewer-1",
		},
	}
	schema := `{"type":"object","required":["decision"],"properties":{"decision":{"type":"string"}}}`
	instruction := renderExecutionReviewDispatchInstruction(delivery, schema)
	for _, expected := range []string{"submission_id: submission-1", "[review_contract]", schema, "operation=review_work", "stable top-level request_id", "without a contract lookup", "trusted ReviewBinding and allowed_actions", "Do not infer review authority from this text"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("missing %q in instruction: %s", expected, instruction)
		}
	}
	if fallback := renderExecutionReviewDispatchInstruction(delivery, ""); strings.Contains(fallback, "[review_contract]") || strings.Contains(fallback, "without a contract lookup") {
		t.Fatalf("unconfigured schema was advertised: %s", fallback)
	}
}

func TestRoomRuntimeSharesDynamicSelfWorkBindingWithCommandAndGraphActor(t *testing.T) {
	service := &Service{}
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

	state := execution.runtimeCommandRoundContext("").CommandContext.WorkBindingState
	if state == nil || state != execution.ensureWorkBindingState() {
		t.Fatalf("Execution command WorkBindingState = %#v", state)
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
			service := &Service{Host: runtimehost.Host{ExecutionContext: terminalizer}}
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

type deletedConversationStore struct{}

func (deletedConversationStore) GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return nil, roomsvc.ErrConversationNotFound
}

func (deletedConversationStore) GetConversationContextForSystem(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return nil, roomsvc.ErrConversationNotFound
}

func (deletedConversationStore) UpdateSessionRuntimeIdentity(context.Context, string, string, string) error {
	return nil
}

func (deletedConversationStore) TouchConversationActivity(context.Context, string, time.Time) error {
	return nil
}

func (deletedConversationStore) MarkConversationStarted(context.Context, string, time.Time) error {
	return nil
}

func (deletedConversationStore) BuildRoomSkillPrompt(context.Context, []string) (string, error) {
	return "", nil
}

// A Room deleted after its WorkGraph stalled must be skipped by startup
// recovery, not reported as a host failure that blocks every later graph.
func TestQueueExecutionRecoveryReportsDeletedConversationAsUnavailable(t *testing.T) {
	service := &Service{rooms: deletedConversationStore{}}

	err := service.QueueExecutionRecovery(context.Background(), protocol.Execution{
		ID: "execution-deleted-room", OwnerUserID: "owner", ScopeKind: protocol.ExecutionScopeRoom,
		RoomID: "room-gone", ConversationID: "conversation-gone", CoordinatorAgentID: "lead",
	}, "startup:execution-deleted-room:1")

	if !errors.Is(err, orchestrationsvc.ErrExecutionRecoveryTargetUnavailable) {
		t.Fatalf("err = %v, want ErrExecutionRecoveryTargetUnavailable", err)
	}
}
