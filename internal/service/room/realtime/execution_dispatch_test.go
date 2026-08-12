package realtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
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
	f.binding = cloneExecutionWorkBinding(binding)
	return f.err
}

func TestManagedExecutionTargetAdmissionRunsBeforeRoomWake(t *testing.T) {
	provider := &managedExecutionAdmissionFake{err: errors.New("target has no current Assignment")}
	service := &Service{executionContext: provider}
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RootRoundID:    "root-1",
		OwnerUserID:    "owner-1",
	}
	err := service.authorizeManagedExecutionTarget(
		context.Background(),
		roundValue,
		"agent-unassigned",
		binding,
	)
	if err == nil {
		t.Fatal("Room wake bypassed managed Execution admission")
	}
	if provider.actor.ExecutionID != "execution-1" ||
		provider.actor.AgentID != "agent-unassigned" ||
		provider.actor.ScopeKind != protocol.ExecutionScopeRoom ||
		provider.binding == nil ||
		provider.binding.AssignmentID != "assignment-1" {
		t.Fatalf("admission identity = actor=%+v binding=%+v", provider.actor, provider.binding)
	}
}

func TestRawMentionRemainsConversationWhenRoomHasManagedExecution(t *testing.T) {
	provider := &managedExecutionAdmissionFake{err: errors.New("target has no current Assignment")}
	service := &Service{executionContext: provider}
	parent := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RootRoundID:    "root-1",
		OwnerUserID:    "owner-1",
	}
	err := service.authorizeManagedExecutionTarget(
		context.Background(),
		parent,
		"agent-unassigned",
		nil,
	)
	if err != nil {
		t.Fatalf("conversation-only raw @ was rejected: %v", err)
	}
	if provider.binding != nil || provider.actor.AgentID != "" {
		t.Fatalf("conversation-only raw @ consulted WorkGraph admission: %+v", provider.actor)
	}
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

	permanent := &Service{executionContext: &managedExecutionAdmissionFake{
		err: &orchestrationsvc.DomainError{
			Code:    orchestrationsvc.ErrorCodeWorkBindingMismatch,
			Message: "the bound Attempt was superseded",
		},
	}}
	if err := permanent.startPublicMentionRoundLocked(
		context.Background(),
		parent,
		[]publicMentionWake{wake},
	); err != nil {
		t.Fatalf("stale structured wake should be consumed as terminal: %v", err)
	}

	transientErr := errors.New("database temporarily unavailable")
	transient := &Service{executionContext: &managedExecutionAdmissionFake{err: transientErr}}
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

func TestRoomExecutionActorRoleUsesHostAndFailsClosedWhenMissing(t *testing.T) {
	tests := []struct {
		name        string
		coordinator string
		actor       string
		want        orchestrationsvc.ExecutionActorRole
	}{
		{
			name:        "host is coordinator",
			coordinator: "agent-host",
			actor:       "agent-host",
			want:        orchestrationsvc.ExecutionActorCoordinator,
		},
		{
			name:        "other member stays member",
			coordinator: "agent-host",
			actor:       "agent-worker",
			want:        orchestrationsvc.ExecutionActorMember,
		},
		{
			name:  "missing host does not elect first slot",
			actor: "agent-worker",
			want:  orchestrationsvc.ExecutionActorMember,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := roomExecutionActorRole(test.coordinator, test.actor); got != test.want {
				t.Fatalf("role = %q, want %q", got, test.want)
			}
		})
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
	instruction := renderExecutionDispatchInstruction(delivery)
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
		renderExecutionDispatchInstruction(delivery),
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
		renderExecutionDispatchInstruction(delivery),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !accepted || receipt.HandoffID != handoffID {
		t.Fatalf("repeat acceptance = %t, receipt=%+v", accepted, receipt)
	}
}
