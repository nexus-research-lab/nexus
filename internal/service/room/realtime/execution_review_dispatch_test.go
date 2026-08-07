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

type managedReviewAdmissionFake struct {
	actor   orchestrationsvc.ActorContext
	binding *protocol.ExecutionReviewBinding
	err     error
}

func (f *managedReviewAdmissionFake) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (f *managedReviewAdmissionFake) AuthorizeRoomReviewReturn(
	_ context.Context,
	actor orchestrationsvc.ActorContext,
	binding *protocol.ExecutionReviewBinding,
) error {
	f.actor = actor
	f.binding = cloneExecutionReviewBinding(binding)
	return f.err
}

func TestExecutionReviewDispatchCarriesTrustedReviewBinding(t *testing.T) {
	delivery := testExecutionReviewDelivery()
	if err := validateExecutionReviewDispatchDelivery(delivery); err != nil {
		t.Fatal(err)
	}
	instruction := renderExecutionReviewDispatchInstruction(delivery)
	for _, expected := range []string{
		"execution_id: execution-1",
		"plan_id: plan-1",
		"work_item_id: work-1",
		"spec_id: spec-1",
		"assignment_id: assignment-1",
		"submission_id: submission-1",
		"review_dispatch_id: review-dispatch-1",
		"target_agent_id: agent-lead",
		"call review_work",
	} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("structured review instruction missing %q:\n%s", expected, instruction)
		}
	}
	incomplete := delivery
	incomplete.Binding.SubmissionID = ""
	if err := validateExecutionReviewDispatchDelivery(incomplete); err == nil {
		t.Fatal("review delivery without Submission binding was accepted")
	}
	conflicting := delivery
	conflicting.TargetAgentID = "agent-other"
	if err := validateExecutionReviewDispatchDelivery(conflicting); err == nil {
		t.Fatal("review delivery target differing from binding was accepted")
	}
}

func TestExecutionReviewDispatchHandoffIsDurableAndIdempotent(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	store := workspacestore.NewRoomPublicHandoffStore(stateRoot)
	service := &Service{publicHandoffs: store}
	delivery := testExecutionReviewDelivery()
	handoffID := "execution_review_dispatch_review-dispatch-1"
	accepted, _, err := service.ensureExecutionReviewDispatchHandoff(
		delivery,
		handoffID,
		renderExecutionReviewDispatchInstruction(delivery),
	)
	if err != nil {
		t.Fatal(err)
	}
	if accepted {
		t.Fatal("new review handoff must still enter the Room delivery path")
	}
	handoff, ok, err := store.Get("owner-1", "conversation-1", handoffID)
	if err != nil || !ok {
		t.Fatalf("durable review handoff = %+v, ok=%t, err=%v", handoff, ok, err)
	}
	if handoff.Status != "source_finished" ||
		handoff.WorkBinding != nil ||
		!executionReviewBindingEqual(handoff.ReviewBinding, &delivery.Binding) {
		t.Fatalf("durable review handoff = %+v", handoff)
	}
	if _, claimed, claimErr := store.Claim(
		"owner-1",
		"conversation-1",
		handoffID,
	); claimErr != nil || !claimed {
		t.Fatalf("claim = %t, err=%v", claimed, claimErr)
	}
	if err = store.MarkStarted(
		"owner-1",
		"conversation-1",
		handoffID,
		"round-1",
	); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending("owner-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 ||
		!executionReviewBindingEqual(
			pending[0].ReviewBinding,
			&delivery.Binding,
		) {
		t.Fatalf("started review handoff must remain recoverable: %+v", pending)
	}
	accepted, receipt, err := service.ensureExecutionReviewDispatchHandoff(
		delivery,
		handoffID,
		renderExecutionReviewDispatchInstruction(delivery),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !accepted || receipt.HandoffID != handoffID {
		t.Fatalf("repeat acceptance = %t, receipt=%+v", accepted, receipt)
	}
}

func TestManagedReviewAdmissionRunsBeforeRoomQueueOrWake(t *testing.T) {
	provider := &managedReviewAdmissionFake{
		err: errors.New("Submission already reviewed"),
	}
	service := &Service{executionContext: provider}
	binding := testExecutionReviewDelivery().Binding
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RootRoundID:    "root-1",
		OwnerUserID:    "owner-1",
	}
	err := service.authorizeManagedExecutionReviewTarget(
		context.Background(),
		roundValue,
		"agent-lead",
		&binding,
	)
	if err == nil {
		t.Fatal("stale review return bypassed managed admission")
	}
	if provider.actor.ExecutionID != "execution-1" ||
		provider.actor.AgentID != "agent-lead" ||
		provider.actor.WorkBinding != nil ||
		provider.actor.ReviewBinding == nil ||
		provider.binding == nil ||
		provider.binding.SubmissionID != "submission-1" {
		t.Fatalf(
			"review admission identity = actor=%+v binding=%+v",
			provider.actor,
			provider.binding,
		)
	}
}

func testExecutionReviewDelivery() orchestrationsvc.ExecutionReviewDispatchDelivery {
	return orchestrationsvc.ExecutionReviewDispatchDelivery{
		OwnerUserID:    "owner-1",
		SessionKey:     "room:group:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		SourceAgentID:  "agent-worker",
		TargetAgentID:  "agent-lead",
		Instruction:    "Review the submitted evidence.",
		Binding: protocol.ExecutionReviewBinding{
			ExecutionID:      "execution-1",
			PlanID:           "plan-1",
			WorkItemID:       "work-1",
			SpecID:           "spec-1",
			AssignmentID:     "assignment-1",
			SubmissionID:     "submission-1",
			ReviewDispatchID: "review-dispatch-1",
			TargetAgentID:    "agent-lead",
		},
	}
}
