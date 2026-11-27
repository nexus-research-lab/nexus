package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestStructuredRoomWorkBindingFailsClosedForHistoricalDependencyOverflow(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	for index := 0; index < protocol.ExecutionProjectionCollectionLimit+1; index++ {
		snapshot.Dependencies = append(snapshot.Dependencies, protocol.ExecutionPlanDependency{
			PlanID:              binding.PlanID,
			ExecutionID:         binding.ExecutionID,
			WorkItemID:          binding.WorkItemID,
			DependsOnWorkItemID: fmt.Sprintf("upstream-%02d", index),
			Kind:                protocol.WorkDependencyHard,
		})
	}
	service := NewService(&fakeRepository{snapshot: snapshot})
	_, err := service.GetSnapshot(
		context.Background(),
		structuredRoomMemberActor(binding),
		binding.ExecutionID,
	)
	if !errors.Is(err, protocol.ErrExecutionProjectionLimitExceeded) {
		t.Fatalf("error = %v", err)
	}
}

func TestStructuredRoomWorkBindingHidesUnacceptedUpstreamPayloadAndAuthority(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	addAcceptedWorkBindingDependency(snapshot)
	snapshot.Acceptances = nil
	snapshot.Submissions[0].ResultSummary = "UNREVIEWED UPSTREAM SECRET"
	snapshot.Submissions[0].ResultRefs = []string{"secret://unaccepted"}
	service := NewService(&fakeRepository{snapshot: snapshot})
	actor := structuredRoomMemberActor(binding)

	scoped, err := service.GetSnapshot(context.Background(), actor, binding.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.WorkItems) != 2 ||
		len(scoped.Submissions) != 0 ||
		len(scoped.Acceptances) != 0 {
		t.Fatalf("unaccepted upstream payload was not filtered: %#v", scoped)
	}
	rendered, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `<dependency kind="hard" status="not_accepted">`) ||
		strings.Contains(rendered, "UNREVIEWED UPSTREAM SECRET") ||
		strings.Contains(rendered, "secret://unaccepted") {
		t.Fatalf("unaccepted upstream context = %s", rendered)
	}

	result, err := service.BlockWork(context.Background(), actor, BlockWorkInput{
		ExecutionID:      binding.ExecutionID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "block-upstream",
		WorkItemID:       "work-2",
		Reason:           "attempted sibling mutation",
		NeededInput:      "forbidden",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected || result.ReasonCode != ErrorCodeWorkBindingMismatch {
		t.Fatalf("upstream mutation escaped WorkBinding fence: %#v", result)
	}
}

func TestRoomCoordinatorSelfReviewContinuesFromWorkBindingSameRound(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Assignments[0].OwnerAgentID = snapshot.Execution.CoordinatorAgentID
	snapshot.Assignments[0].ReturnToAgentID = snapshot.Execution.CoordinatorAgentID
	snapshot.Assignments[0].Strategy = protocol.AssignmentStrategySelf
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Dispatches = nil
	snapshot.Attempts[0].DispatchID = ""
	snapshot.Attempts[0].ExecutorAgentID = snapshot.Execution.CoordinatorAgentID
	binding.DispatchID = ""
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-self-review",
		ExecutionID:      binding.ExecutionID,
		PlanID:           binding.PlanID,
		WorkItemID:       binding.WorkItemID,
		SpecID:           binding.SpecID,
		AssignmentID:     binding.AssignmentID,
		AttemptID:        binding.AttemptID,
		SubmitterAgentID: snapshot.Execution.CoordinatorAgentID,
		ResultSummary:    "Lead-owned evidence is ready",
	}}
	snapshot.CompletionBlockers = []string{"another selected Work Item remains"}

	repository := &fakeRepository{snapshot: snapshot}
	repository.review = func(
		_ context.Context,
		command orchestrationstore.ReviewCommand,
	) (*protocol.ExecutionSnapshot, error) {
		result := cloneExecutionSnapshot(repository.snapshot)
		result.Execution.Version++
		result.Acceptances = append(result.Acceptances, command.Acceptance)
		repository.snapshot = result
		return result, nil
	}
	service := NewService(repository)
	bindingCopy := binding
	actor := ActorContext{
		OwnerUserID:    snapshot.Execution.OwnerUserID,
		SessionKey:     snapshot.Execution.SessionKey,
		ExecutionID:    binding.ExecutionID,
		WorkBinding:    &bindingCopy,
		AgentID:        snapshot.Execution.CoordinatorAgentID,
		Role:           ExecutionActorCoordinator,
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         snapshot.Execution.RoomID,
		ConversationID: snapshot.Execution.ConversationID,
		RootRoundID:    "root-self-review",
		RuntimeRoundID: "runtime-self-review",
		AgentRoundID:   "agent-self-review",
	}

	beforeReview, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(beforeReview, `<lane type="work" />`) ||
		!strings.Contains(beforeReview, `<action>review_work</action>`) {
		t.Fatalf("self-review WorkBinding context = %s", beforeReview)
	}

	reviewed, err := service.ReviewWork(context.Background(), actor, ReviewWorkInput{
		ExecutionID:      binding.ExecutionID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "review-own-work-same-round",
		Decision:         protocol.WorkAcceptanceAccepted,
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "sources cited",
			Passed:    true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Outcome != MutationApplied ||
		!service.runtimeCoordinationActive(actor, binding.ExecutionID) {
		t.Fatalf("self-review did not return to same-round coordination: %+v", reviewed)
	}
	afterReview, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(afterReview, `<lane type="coordination" />`) {
		t.Fatalf("self-review continuation context = %s", afterReview)
	}
}

func TestRoomExactGoalContinuationEntersCoordinationWithoutConversationBootstrap(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Execution.GoalID = "goal-room"
	snapshot.Execution.GoalObjectiveRevision = 4
	service := NewService(&fakeRepository{snapshot: snapshot})
	actor := structuredRoomMemberActor(binding)
	actor.AgentID = snapshot.Execution.CoordinatorAgentID
	actor.Role = ExecutionActorCoordinator
	actor.WorkBinding = nil
	actor.GoalID = snapshot.Execution.GoalID
	actor.GoalObjectiveRevision = snapshot.Execution.GoalObjectiveRevision

	rendered, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `<lane type="coordination" />`) ||
		!strings.Contains(rendered, `<goal id="goal-room"`) {
		t.Fatalf("Goal continuation context = %s", rendered)
	}
	actor.ExecutionID = "execution-other"
	if _, err = service.RuntimeContext(context.Background(), actor); err == nil {
		t.Fatal("Goal continuation with another Execution identity was accepted")
	} else {
		var domainErr *DomainError
		if !errors.As(err, &domainErr) ||
			domainErr.Code != ErrorCodeGoalBindingConflict {
			t.Fatalf("wrong Execution Goal continuation error = %v", err)
		}
	}
	actor.ExecutionID = snapshot.Execution.ID
	actor.GoalObjectiveRevision--
	if _, err = service.RuntimeContext(context.Background(), actor); err == nil {
		t.Fatal("stale Goal continuation revision was accepted")
	} else {
		var domainErr *DomainError
		if !errors.As(err, &domainErr) ||
			domainErr.Code != ErrorCodeGoalBindingConflict {
			t.Fatalf("stale Goal continuation error = %v", err)
		}
	}
}

func structuredRoomWorkBindingSnapshot() (
	*protocol.ExecutionSnapshot,
	protocol.ExecutionWorkBinding,
) {
	snapshot := assignedExecutionSnapshot()
	addSecondDelegableAssignment(snapshot)
	snapshot.Execution.ScopeKind = protocol.ExecutionScopeRoom
	snapshot.Execution.RoomID = "room-1"
	snapshot.Execution.ConversationID = "conversation-1"
	snapshot.Execution.CoordinatorAgentID = "agent-lead"
	snapshot.Assignments[1].Strategy = protocol.AssignmentStrategyRoomMember
	snapshot.Attempts[0].DispatchID = "dispatch-1"
	snapshot.Attempts[1].DispatchID = "dispatch-2"
	snapshot.Dispatches = []protocol.ExecutionDispatch{
		{
			ID:            "dispatch-1",
			ExecutionID:   snapshot.Execution.ID,
			PlanID:        "plan-1",
			WorkItemID:    "work-1",
			SpecID:        "spec-1",
			AssignmentID:  "assignment-1",
			TargetAgentID: "agent-worker",
			Status:        protocol.ExecutionDispatchStatusDelivered,
		},
		{
			ID:            "dispatch-2",
			ExecutionID:   snapshot.Execution.ID,
			PlanID:        "plan-1",
			WorkItemID:    "work-2",
			SpecID:        "spec-2",
			AssignmentID:  "assignment-2",
			TargetAgentID: "agent-worker",
			Status:        protocol.ExecutionDispatchStatusDelivered,
		},
	}
	return snapshot, protocol.ExecutionWorkBinding{
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
}

func structuredRoomMemberActor(binding protocol.ExecutionWorkBinding) ActorContext {
	bindingCopy := binding
	return ActorContext{
		OwnerUserID:    "owner-1",
		SessionKey:     "session-1",
		ExecutionID:    binding.ExecutionID,
		WorkBinding:    &bindingCopy,
		AgentID:        "agent-worker",
		Role:           ExecutionActorMember,
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RootRoundID:    "root-round-1",
		RuntimeRoundID: "agent-round-1",
		AgentRoundID:   "agent-round-1",
	}
}

func addAcceptedWorkBindingDependency(snapshot *protocol.ExecutionSnapshot) {
	snapshot.WorkItemSpecs[0].InputRefs = []string{
		"artifact://accepted-input",
		"brief://work-1",
	}
	snapshot.Dependencies = []protocol.ExecutionPlanDependency{{
		PlanID:              "plan-1",
		ExecutionID:         snapshot.Execution.ID,
		WorkItemID:          "work-1",
		DependsOnWorkItemID: "work-2",
		Kind:                protocol.WorkDependencyHard,
	}}
	snapshot.OutputClaims = []protocol.ExecutionPlanOutputClaim{
		{
			PlanID:      "plan-1",
			ExecutionID: snapshot.Execution.ID,
			WorkItemID:  "work-1",
			SpecID:      "spec-1",
			Scope:       "file:reports/work-1.md",
			Mode:        protocol.WorkOutputScopeExclusive,
		},
		{
			PlanID:      "plan-1",
			ExecutionID: snapshot.Execution.ID,
			WorkItemID:  "work-2",
			SpecID:      "spec-2",
			Scope:       "file:artifacts/upstream.json",
			Mode:        protocol.WorkOutputScopeExclusive,
		},
	}
	snapshot.Assignments[1].Status = protocol.WorkAssignmentStatusCompleted
	snapshot.Attempts[1].Status = protocol.WorkAttemptStatusSucceeded
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-upstream",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           "plan-1",
		WorkItemID:       "work-2",
		SpecID:           "spec-2",
		AssignmentID:     "assignment-2",
		AttemptID:        "attempt-2",
		Sequence:         1,
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "Accepted upstream facts",
		ResultRefs:       []string{"artifact://upstream"},
		Evidence:         []string{"evidence://upstream"},
	}}
	snapshot.Acceptances = []protocol.WorkAcceptance{{
		ID:           "acceptance-upstream",
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       "plan-1",
		WorkItemID:   "work-2",
		SpecID:       "spec-2",
		AssignmentID: "assignment-2",
		SubmissionID: "submission-upstream",
		Decision:     protocol.WorkAcceptanceAccepted,
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "claims supported",
			Passed:    true,
			Evidence:  []string{"evidence://upstream"},
		}},
	}}
}
