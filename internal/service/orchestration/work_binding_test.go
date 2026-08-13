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

func TestStructuredRoomWorkBindingScopesSnapshotAndRejectsCrossAssignmentMutation(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	repository := &fakeRepository{snapshot: snapshot}
	service := NewService(repository)
	actor := structuredRoomMemberActor(binding)

	scoped, err := service.GetSnapshot(context.Background(), actor, binding.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.WorkItems) != 1 ||
		scoped.WorkItems[0].ID != binding.WorkItemID ||
		len(scoped.Assignments) != 1 ||
		scoped.Assignments[0].ID != binding.AssignmentID ||
		len(scoped.Dispatches) != 1 ||
		scoped.Dispatches[0].ID != binding.DispatchID {
		t.Fatalf("bound snapshot was not capability-scoped: %#v", scoped)
	}
	if len(repository.snapshot.Assignments) != 2 {
		t.Fatal("scoping mutated the repository snapshot")
	}

	submitResult, err := service.SubmitWork(context.Background(), actor, SubmitWorkInput{
		ExecutionID:      binding.ExecutionID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "submit-cross-assignment",
		WorkItemID:       "work-2",
		AssignmentID:     "assignment-2",
		ResultSummary:    "attempted cross-assignment result",
	})
	if err != nil {
		t.Fatal(err)
	}
	if submitResult.Outcome != MutationRejected ||
		submitResult.ReasonCode != ErrorCodeInvalidInput {
		t.Fatalf("cross-assignment submit result = %#v", submitResult)
	}

	blockResult, err := service.BlockWork(context.Background(), actor, BlockWorkInput{
		ExecutionID:      binding.ExecutionID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "block-cross-assignment",
		WorkItemID:       "work-2",
		Reason:           "missing input",
		NeededInput:      "external evidence",
	})
	if err != nil {
		t.Fatal(err)
	}
	if blockResult.Outcome != MutationRejected ||
		blockResult.ReasonCode != ErrorCodeInvalidInput {
		t.Fatalf("cross-assignment block result = %#v", blockResult)
	}
}

func TestRoomSelfWorkBindingNeedsNoDispatchButStillScopesOneAssignment(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Assignments[0].Strategy = protocol.AssignmentStrategySelf
	snapshot.Assignments[0].OwnerAgentID = snapshot.Execution.CoordinatorAgentID
	snapshot.Assignments[0].ReturnToAgentID = snapshot.Execution.CoordinatorAgentID
	snapshot.Dispatches = nil
	snapshot.Attempts[0].DispatchID = ""
	snapshot.Attempts[0].ExecutorAgentID = snapshot.Execution.CoordinatorAgentID
	binding.DispatchID = ""
	actor := structuredRoomMemberActor(binding)
	actor.AgentID = snapshot.Execution.CoordinatorAgentID
	actor.Role = ExecutionActorCoordinator

	scoped, err := NewService(&fakeRepository{snapshot: snapshot}).GetSnapshot(
		context.Background(),
		actor,
		binding.ExecutionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.Assignments) != 1 || scoped.Assignments[0].ID != binding.AssignmentID ||
		len(scoped.Dispatches) != 0 || len(scoped.Attempts) != 1 {
		t.Fatalf("self WorkBinding scope = %#v", scoped)
	}

	bad := binding
	bad.DispatchID = "model-forged-dispatch"
	actor.WorkBinding = &bad
	if _, err = NewService(&fakeRepository{snapshot: snapshot}).GetSnapshot(
		context.Background(), actor, binding.ExecutionID,
	); err == nil || !strings.Contains(err.Error(), "must not carry a Dispatch") {
		t.Fatalf("forged self dispatch error = %v", err)
	}
}

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

func TestStructuredRoomWorkBindingFailsClosedForHistoricalContractOverflow(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	for index := range snapshot.WorkItemSpecs {
		if snapshot.WorkItemSpecs[index].ID == binding.SpecID {
			snapshot.WorkItemSpecs[index].InputRefs = make(
				[]string,
				protocol.ExecutionProjectionCollectionLimit+1,
			)
			break
		}
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

func TestStructuredRoomWorkBindingKeepsOnlyAcceptedUpstreamReadProjection(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	addAcceptedWorkBindingDependency(snapshot)
	snapshot.CompletionBlockers = []string{"UNRELATED SIBLING BLOCKER"}
	snapshot.ReviewDispatches = []protocol.ExecutionReviewDispatch{
		{ID: "review-own", AssignmentID: binding.AssignmentID},
		{ID: "review-upstream", AssignmentID: "assignment-2"},
	}
	snapshot.CancellationDispatches = []protocol.ExecutionCancellationDispatch{
		{ID: "cancel-own", AssignmentID: binding.AssignmentID},
		{ID: "cancel-upstream", AssignmentID: "assignment-2"},
	}
	service := NewService(&fakeRepository{snapshot: snapshot})
	actor := structuredRoomMemberActor(binding)

	scoped, err := service.GetSnapshot(context.Background(), actor, binding.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.WorkItems) != 2 ||
		len(scoped.WorkItemSpecs) != 2 ||
		len(scoped.PlanItems) != 2 ||
		len(scoped.Dependencies) != 1 ||
		len(scoped.OutputClaims) != 2 {
		t.Fatalf("upstream read projection is incomplete: %#v", scoped)
	}
	if len(scoped.WorkItemStates) != 1 ||
		scoped.WorkItemStates[0].WorkItemID != binding.WorkItemID ||
		len(scoped.Assignments) != 1 ||
		scoped.Assignments[0].ID != binding.AssignmentID ||
		len(scoped.Dispatches) != 1 ||
		scoped.Dispatches[0].ID != binding.DispatchID ||
		len(scoped.Attempts) != 1 ||
		scoped.Attempts[0].AssignmentID != binding.AssignmentID {
		t.Fatalf("upstream live responsibility leaked: %#v", scoped)
	}
	if len(scoped.Submissions) != 1 ||
		scoped.Submissions[0].ID != "submission-upstream" ||
		len(scoped.Acceptances) != 1 ||
		scoped.Acceptances[0].ID != "acceptance-upstream" {
		t.Fatalf("accepted upstream delivery is missing: %#v", scoped)
	}
	if len(scoped.ReviewDispatches) != 1 ||
		scoped.ReviewDispatches[0].ID != "review-own" ||
		len(scoped.CancellationDispatches) != 1 ||
		scoped.CancellationDispatches[0].ID != "cancel-own" ||
		len(scoped.CompletionBlockers) != 0 {
		t.Fatalf("sibling control state leaked: %#v", scoped)
	}

	rendered, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`<ref>artifact://accepted-input</ref>`,
		`<scope mode="exclusive">file:reports/work-1.md</scope>`,
		`<dependency kind="hard" status="accepted">`,
		`<upstream work_item_id="work-2" logical_key="analysis" spec_id="spec-2" />`,
		`<accepted_submission id="submission-upstream">`,
		`<result_summary>Accepted upstream facts</result_summary>`,
		`<acceptance id="acceptance-upstream">`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("bound runtime context missing %q:\n%s", expected, rendered)
		}
	}
	for _, forbidden := range []string{
		`assignment_id="assignment-2"`,
		`dispatch_id="dispatch-2"`,
		"UNRELATED SIBLING BLOCKER",
	} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("bound runtime context leaked %q:\n%s", forbidden, rendered)
		}
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
	if result.Outcome != MutationRejected || result.ReasonCode != ErrorCodeWrongOwner {
		t.Fatalf("upstream mutation escaped WorkBinding fence: %#v", result)
	}
}

func TestStructuredRoomWorkBindingRejectsStaleOrInputSelectedIdentity(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	service := NewService(&fakeRepository{snapshot: snapshot})

	for _, testCase := range []struct {
		name        string
		actor       ActorContext
		executionID string
	}{
		{
			name:        "tool input selects another Execution",
			actor:       structuredRoomMemberActor(binding),
			executionID: "execution-other",
		},
		{
			name: "binding mixes another Assignment",
			actor: func() ActorContext {
				actor := structuredRoomMemberActor(binding)
				actor.WorkBinding.AssignmentID = "assignment-2"
				return actor
			}(),
			executionID: binding.ExecutionID,
		},
		{
			name: "binding changes the root Attempt",
			actor: func() ActorContext {
				actor := structuredRoomMemberActor(binding)
				actor.WorkBinding.AttemptID = "attempt-2"
				return actor
			}(),
			executionID: binding.ExecutionID,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.GetSnapshot(
				context.Background(),
				testCase.actor,
				testCase.executionID,
			)
			var domainErr *DomainError
			if !errors.As(err, &domainErr) ||
				domainErr.Code != ErrorCodeWorkBindingMismatch {
				t.Fatalf("GetSnapshot error = %v, want %s", err, ErrorCodeWorkBindingMismatch)
			}
		})
	}
}

func TestStructuredRoomWorkBindingReportsRetargetedPredecessorAsTerminal(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	snapshot.Plan = nil
	service := NewService(&fakeRepository{snapshot: snapshot})

	_, err := service.GetSnapshot(
		context.Background(),
		structuredRoomMemberActor(binding),
		binding.ExecutionID,
	)
	var domainErr *DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != ErrorCodeExecutionTerminal {
		t.Fatalf("GetSnapshot error = %v, want %s", err, ErrorCodeExecutionTerminal)
	}
	if !strings.Contains(domainErr.Message, "fresh Assignment") {
		t.Fatalf("terminal guidance = %q, want fresh Assignment recovery", domainErr.Message)
	}
}

func TestStructuredRoomReviewBindingAdmitsOnlyItsSelectedPendingReview(t *testing.T) {
	snapshot, dispatch := reviewReturnSnapshot()
	binding := &protocol.ExecutionReviewBinding{
		ExecutionID:      dispatch.ExecutionID,
		PlanID:           dispatch.PlanID,
		WorkItemID:       dispatch.WorkItemID,
		SpecID:           dispatch.SpecID,
		AssignmentID:     dispatch.AssignmentID,
		SubmissionID:     dispatch.SubmissionID,
		ReviewDispatchID: dispatch.ID,
		TargetAgentID:    dispatch.TargetAgentID,
	}
	service := NewService(&fakeRepository{snapshot: snapshot})
	actor := ActorContext{
		OwnerUserID:    snapshot.Execution.OwnerUserID,
		SessionKey:     snapshot.Execution.SessionKey,
		ExecutionID:    snapshot.Execution.ID,
		ReviewBinding:  binding,
		AgentID:        snapshot.Execution.CoordinatorAgentID,
		Role:           ExecutionActorCoordinator,
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         snapshot.Execution.RoomID,
		ConversationID: snapshot.Execution.ConversationID,
		RuntimeRoundID: "pending-review-round",
	}
	loaded, err := service.GetSnapshot(
		context.Background(),
		actor,
		snapshot.Execution.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Submissions) != 1 ||
		len(loaded.ReviewDispatches) != 1 ||
		len(loaded.Assignments) != 1 {
		t.Fatalf("selected reviewer lost its bounded review snapshot: %+v", loaded)
	}
	if err = service.mintRuntimeCoordination(actor, snapshot.Execution.ID); err != nil {
		t.Fatal(err)
	}
	pendingContext, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pendingContext, `<lane type="review" />`) {
		t.Fatalf("unresolved ReviewBinding was hidden by generic coordination:\n%s", pendingContext)
	}
	service.ReleaseRuntimeCoordination(actor)

	wrongTarget := actor
	wrongBinding := *binding
	wrongBinding.TargetAgentID = "agent-other"
	wrongTarget.ReviewBinding = &wrongBinding
	if _, err = service.GetSnapshot(
		context.Background(),
		wrongTarget,
		snapshot.Execution.ID,
	); err == nil {
		t.Fatal("review binding targeting another Agent was admitted")
	}

	terminalSnapshot := cloneExecutionSnapshot(snapshot)
	terminalSnapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	terminalService := NewService(&fakeRepository{snapshot: terminalSnapshot})
	if _, err = terminalService.GetSnapshot(
		context.Background(),
		actor,
		snapshot.Execution.ID,
	); err == nil {
		t.Fatal("terminal Execution review binding was admitted")
	}

	reviewedSnapshot := cloneExecutionSnapshot(snapshot)
	reviewedSnapshot.Acceptances = []protocol.WorkAcceptance{{
		ID:           "acceptance-1",
		ExecutionID:  dispatch.ExecutionID,
		PlanID:       dispatch.PlanID,
		WorkItemID:   dispatch.WorkItemID,
		SpecID:       dispatch.SpecID,
		AssignmentID: dispatch.AssignmentID,
		SubmissionID: dispatch.SubmissionID,
		Decision:     protocol.WorkAcceptanceAccepted,
		ReviewerKind: protocol.WorkReviewerAgent,
		ReviewerID:   dispatch.TargetAgentID,
	}}
	reviewedService := NewService(&fakeRepository{snapshot: reviewedSnapshot})
	if _, err = reviewedService.GetSnapshot(
		context.Background(),
		actor,
		snapshot.Execution.ID,
	); err == nil {
		t.Fatal("already reviewed Submission binding was admitted")
	}
}

func TestStructuredRoomReviewBindingAllowsSelectedMemberReviewer(t *testing.T) {
	snapshot, dispatch := reviewReturnSnapshot()
	dispatch.TargetAgentID = "agent-reviewer"
	snapshot.Assignments[0].ReturnToAgentID = dispatch.TargetAgentID
	snapshot.ReviewDispatches[0] = dispatch
	actor := ActorContext{
		OwnerUserID:    snapshot.Execution.OwnerUserID,
		SessionKey:     snapshot.Execution.SessionKey,
		ExecutionID:    snapshot.Execution.ID,
		AgentID:        dispatch.TargetAgentID,
		Role:           ExecutionActorMember,
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         snapshot.Execution.RoomID,
		ConversationID: snapshot.Execution.ConversationID,
		ReviewBinding: &protocol.ExecutionReviewBinding{
			ExecutionID:      dispatch.ExecutionID,
			PlanID:           dispatch.PlanID,
			WorkItemID:       dispatch.WorkItemID,
			SpecID:           dispatch.SpecID,
			AssignmentID:     dispatch.AssignmentID,
			SubmissionID:     dispatch.SubmissionID,
			ReviewDispatchID: dispatch.ID,
			TargetAgentID:    dispatch.TargetAgentID,
		},
	}
	loaded, err := NewService(&fakeRepository{snapshot: snapshot}).GetSnapshot(
		context.Background(),
		actor,
		snapshot.Execution.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Assignments) != 1 ||
		len(loaded.Submissions) != 1 ||
		len(loaded.ReviewDispatches) != 1 {
		t.Fatalf("member reviewer snapshot = %+v", loaded)
	}
}

func TestRoomReviewAcceptanceContinuesCoordinationAndAssignsReadyWorkSameRound(t *testing.T) {
	snapshot, dispatch := reviewReturnSnapshot()
	snapshot.WorkItems = append(snapshot.WorkItems, protocol.WorkItem{
		ID:          "work-next",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "verify-sources",
		Kind:        protocol.WorkItemKindVerify,
	})
	snapshot.WorkItemStates = append(snapshot.WorkItemStates, protocol.WorkItemState{
		WorkItemID:    "work-next",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-next",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	})
	snapshot.WorkItemSpecs = append(snapshot.WorkItemSpecs, protocol.WorkItemSpec{
		ID:                 "spec-next",
		WorkItemID:         "work-next",
		ExecutionID:        snapshot.Execution.ID,
		Version:            1,
		Subject:            "Verify sources",
		Objective:          "Verify accepted source evidence",
		Deliverable:        "Verified source set",
		AcceptanceCriteria: []string{"sources verified"},
	})
	snapshot.PlanItems = append(snapshot.PlanItems, protocol.ExecutionPlanItem{
		PlanID:      snapshot.Plan.ID,
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-next",
		SpecID:      "spec-next",
		Required:    true,
		Position:    1,
	})

	repository := &fakeRepository{snapshot: snapshot}
	repository.review = func(
		_ context.Context,
		command orchestrationstore.ReviewCommand,
	) (*protocol.ExecutionSnapshot, error) {
		result := cloneExecutionSnapshot(repository.snapshot)
		result.Execution.Version++
		result.Assignments[0].Status = protocol.WorkAssignmentStatusCompleted
		result.Assignments[0].Version++
		result.Acceptances = []protocol.WorkAcceptance{command.Acceptance}
		result.ReadyWorkItemIDs = []string{"work-next"}
		result.CompletionBlockers = []string{
			"work_item:work-next:required_not_accepted",
		}
		repository.snapshot = result
		return result, nil
	}
	assignCalled := false
	repository.assign = func(
		_ context.Context,
		command orchestrationstore.AssignCommand,
	) (*protocol.ExecutionSnapshot, error) {
		assignCalled = true
		if command.Assignment.WorkItemID != "work-next" ||
			command.Assignment.OwnerAgentID != "agent-analyst" ||
			command.Dispatch == nil ||
			command.Dispatch.Kind != protocol.ExecutionDispatchRoomDirected ||
			command.Dispatch.TargetAgentID != "agent-analyst" {
			t.Fatalf("continuation Assignment = %+v", command)
		}
		result := cloneExecutionSnapshot(repository.snapshot)
		result.Execution.Version++
		result.Assignments = append(result.Assignments, command.Assignment)
		result.Attempts = append(result.Attempts, *command.RootAttempt)
		result.Dispatches = append(result.Dispatches, *command.Dispatch)
		result.ReadyWorkItemIDs = nil
		repository.snapshot = result
		return result, nil
	}
	service := NewService(repository)
	service.newID = func(kind string) string {
		return kind + "-continuation"
	}
	service.SetAssignmentTargetAuthorizer(assignmentTargetAuthorizerFunc(func(
		_ context.Context,
		request AssignmentTargetRequest,
	) error {
		if request.TargetAgentID != "agent-analyst" ||
			request.RoomID != snapshot.Execution.RoomID {
			t.Fatalf("assignment target request = %+v", request)
		}
		return nil
	}))
	actor := ActorContext{
		OwnerUserID: snapshot.Execution.OwnerUserID,
		SessionKey:  snapshot.Execution.SessionKey,
		ExecutionID: snapshot.Execution.ID,
		ReviewBinding: &protocol.ExecutionReviewBinding{
			ExecutionID:      dispatch.ExecutionID,
			PlanID:           dispatch.PlanID,
			WorkItemID:       dispatch.WorkItemID,
			SpecID:           dispatch.SpecID,
			AssignmentID:     dispatch.AssignmentID,
			SubmissionID:     dispatch.SubmissionID,
			ReviewDispatchID: dispatch.ID,
			TargetAgentID:    dispatch.TargetAgentID,
		},
		AgentID:        snapshot.Execution.CoordinatorAgentID,
		Role:           ExecutionActorCoordinator,
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         snapshot.Execution.RoomID,
		ConversationID: snapshot.Execution.ConversationID,
		RootRoundID:    "root-review",
		RuntimeRoundID: "runtime-review",
		AgentRoundID:   "agent-review",
	}

	beforeReview, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(beforeReview, `<lane type="review" />`) {
		t.Fatalf("review return did not enter review lane:\n%s", beforeReview)
	}

	reviewed, err := service.ReviewWork(context.Background(), actor, ReviewWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "review-and-continue",
		SubmissionID:     dispatch.SubmissionID,
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
		!service.runtimeCoordinationActive(actor, snapshot.Execution.ID) {
		t.Fatalf("review did not activate same-round coordination: %+v", reviewed)
	}
	if len(reviewed.NextActions) == 0 ||
		reviewed.NextActions[0].Tool != "assign_work" ||
		reviewed.NextActions[0].WorkItemID != "work-next" {
		t.Fatalf("review continuation next actions = %+v", reviewed.NextActions)
	}
	afterReview, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(afterReview, `<lane type="coordination" />`) ||
		!strings.Contains(afterReview, `<action>assign_work</action>`) ||
		!strings.Contains(afterReview, `logical_key="verify-sources"`) {
		t.Fatalf("review continuation context = %s", afterReview)
	}

	assigned, err := service.AssignWork(context.Background(), actor, AssignWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: reviewed.Snapshot.Execution.Version,
		CommandID:        "assign-ready-after-review",
		WorkItemID:       "work-next",
		TargetAgentID:    "agent-analyst",
		Strategy:         protocol.AssignmentStrategyRoomMember,
		DispatchKind:     protocol.ExecutionDispatchRoomDirected,
	})
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Outcome != MutationApplied || !assignCalled {
		t.Fatalf("same-round downstream assignment = %+v called=%t", assigned, assignCalled)
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
		SubmissionID:     "submission-self-review",
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

func TestStructuredRoomWorkBindingSelectsOneSubagentCandidateAndDynamicContext(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	repository := &fakeRepository{snapshot: snapshot}
	repository.startAttempt = func(
		_ context.Context,
		command orchestrationstore.StartAttemptCommand,
	) (*protocol.ExecutionSnapshot, error) {
		result := cloneExecutionSnapshot(repository.snapshot)
		result.Execution.Version++
		for index := range result.Assignments {
			if result.Assignments[index].ID == command.Attempt.AssignmentID {
				result.Assignments[index].Status = protocol.WorkAssignmentStatusActive
				result.Assignments[index].Version++
			}
		}
		replaced := false
		for index := range result.Attempts {
			if result.Attempts[index].ID == command.Attempt.ID {
				result.Attempts[index] = command.Attempt
				result.Attempts[index].Version++
				replaced = true
				break
			}
		}
		if !replaced {
			child := command.Attempt
			child.Version = 1
			result.Attempts = append(result.Attempts, child)
		}
		repository.snapshot = result
		return result, nil
	}
	service := NewService(repository)
	service.newID = func(kind string) string {
		if kind == "attempt" {
			return "attempt-child-bound"
		}
		return kind + "-generated"
	}
	actor := structuredRoomMemberActor(binding)

	rendered, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		rendered,
		`candidate_assignment_count="1" binding_mode="managed" assignment_id="assignment-1"`,
	) ||
		strings.Contains(rendered, `assignment_id="assignment-2"`) {
		t.Fatalf("bound dynamic context = %s", rendered)
	}

	result, err := service.AdmitSubagentLaunch(
		context.Background(),
		actor,
		SubagentLaunchInput{ToolUseID: "tool-bound"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed ||
		result.Binding == nil ||
		result.Binding.AssignmentID != binding.AssignmentID ||
		result.Binding.ParentAttemptID != binding.AttemptID {
		t.Fatalf("bound subagent admission = %#v", result)
	}

	unboundActor := actor
	unboundActor.WorkBinding = nil
	unboundResult, err := service.AdmitSubagentLaunch(
		context.Background(),
		unboundActor,
		SubagentLaunchInput{ToolUseID: "tool-unbound"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !unboundResult.Allowed ||
		unboundResult.Mode != SubagentAdmissionRuntimeOnly ||
		unboundResult.Binding != nil {
		t.Fatalf("unbound multi-assignment admission = %#v", unboundResult)
	}
}

func TestRoomCoordinatorKeepsAuthorityWhileUnboundMemberStaysConversationOnly(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	service := NewService(&fakeRepository{snapshot: snapshot})

	coordinator := structuredRoomMemberActor(binding)
	coordinator.AgentID = snapshot.Execution.CoordinatorAgentID
	coordinator.Role = ExecutionActorCoordinator
	coordinator.WorkBinding = nil
	coordinatorSnapshot, err := service.GetSnapshot(
		context.Background(),
		coordinator,
		binding.ExecutionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(coordinatorSnapshot.Assignments) != 2 {
		t.Fatalf("coordinator snapshot assignments = %d", len(coordinatorSnapshot.Assignments))
	}
	conversationCoordinator := coordinator
	conversationCoordinator.WorkBinding = nil
	rendered, err := service.RuntimeContext(
		context.Background(),
		conversationCoordinator,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `role="coordinator" lane="conversation"`) ||
		!strings.Contains(rendered, `<coordination_transition available="true">`) ||
		!strings.Contains(rendered, `<action>plan_execution</action>`) ||
		strings.Contains(rendered, "<assigned_work>") {
		t.Fatalf("unbound Room coordinator context = %s", rendered)
	}
	explicitSnapshot, err := service.GetSnapshot(
		context.Background(),
		conversationCoordinator,
		binding.ExecutionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(explicitSnapshot.Assignments) != 2 {
		t.Fatalf(
			"explicit coordinator inspection assignments = %d",
			len(explicitSnapshot.Assignments),
		)
	}

	unbound := structuredRoomMemberActor(binding)
	unbound.WorkBinding = nil
	_, err = service.GetSnapshot(
		context.Background(),
		unbound,
		binding.ExecutionID,
	)
	var domainErr *DomainError
	if !errors.As(err, &domainErr) ||
		domainErr.Code != ErrorCodeConversationOnly {
		t.Fatalf("unbound Room member snapshot error = %v", err)
	}
	rendered, err = service.RuntimeContext(context.Background(), unbound)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `lane="conversation"`) ||
		strings.Contains(rendered, "<assigned_work>") {
		t.Fatalf("unbound Room member context = %s", rendered)
	}
}

func TestRoomConversationCoordinatorRequiresExplicitRoundCoordinationCapability(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	service := NewService(&fakeRepository{snapshot: snapshot})
	actor := structuredRoomMemberActor(binding)
	actor.AgentID = snapshot.Execution.CoordinatorAgentID
	actor.Role = ExecutionActorCoordinator
	actor.WorkBinding = nil

	_, rejected, err := service.mutableSnapshot(
		context.Background(),
		actor,
		snapshot.Execution.ID,
		snapshot.Execution.Version,
		true,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rejected == nil || rejected.ReasonCode != ErrorCodeConversationOnly {
		t.Fatalf("unbound coordinator mutation = %#v", rejected)
	}
	if err = service.ActivateRuntimeCoordination(
		context.Background(),
		actor,
		snapshot,
	); err != nil {
		t.Fatal(err)
	}
	rendered, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `<lane type="coordination" />`) ||
		!strings.Contains(rendered, "<assigned_work>") {
		t.Fatalf("activated coordinator context = %s", rendered)
	}
	_, rejected, err = service.mutableSnapshot(
		context.Background(),
		actor,
		snapshot.Execution.ID,
		snapshot.Execution.Version,
		true,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rejected != nil {
		t.Fatalf("activated coordinator mutation rejected = %#v", rejected)
	}
	service.ReleaseRuntimeCoordination(actor)
	_, rejected, err = service.mutableSnapshot(
		context.Background(),
		actor,
		snapshot.Execution.ID,
		snapshot.Execution.Version,
		true,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rejected == nil || rejected.ReasonCode != ErrorCodeConversationOnly {
		t.Fatalf("released coordinator mutation = %#v", rejected)
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

func TestRoomExactGoalBoundWorkerKeepsScopedWorkCapability(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Execution.GoalID = "goal-room"
	snapshot.Execution.GoalObjectiveRevision = 4
	service := NewService(&fakeRepository{snapshot: snapshot})
	actor := structuredRoomMemberActor(binding)
	actor.GoalID = snapshot.Execution.GoalID
	actor.GoalObjectiveRevision = snapshot.Execution.GoalObjectiveRevision

	rendered, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `<lane type="work" />`) ||
		!strings.Contains(rendered, `<goal id="goal-room"`) ||
		!strings.Contains(rendered, `assignment_id="assignment-1"`) ||
		strings.Contains(rendered, `assignment_id="assignment-2"`) {
		t.Fatalf("Goal-bound worker context = %s", rendered)
	}

	actor.GoalObjectiveRevision--
	if _, err = service.RuntimeContext(context.Background(), actor); err == nil {
		t.Fatal("stale Goal-bound worker revision was accepted")
	} else {
		var domainErr *DomainError
		if !errors.As(err, &domainErr) ||
			domainErr.Code != ErrorCodeGoalBindingConflict {
			t.Fatalf("stale Goal-bound worker error = %v", err)
		}
	}

	actor.GoalObjectiveRevision = snapshot.Execution.GoalObjectiveRevision
	actor.WorkBinding = nil
	if _, err = service.RuntimeContext(context.Background(), actor); err == nil {
		t.Fatal("unbound non-coordinator reused the Room Goal capability")
	} else {
		var domainErr *DomainError
		if !errors.As(err, &domainErr) ||
			domainErr.Code != ErrorCodeGoalBindingConflict {
			t.Fatalf("unbound Room member Goal error = %v", err)
		}
	}
}

func TestRoomExactGoalBoundMemberReviewerKeepsScopedReviewCapability(t *testing.T) {
	snapshot, dispatch := reviewReturnSnapshot()
	dispatch.TargetAgentID = "agent-reviewer"
	snapshot.Assignments[0].ReturnToAgentID = dispatch.TargetAgentID
	snapshot.ReviewDispatches[0] = dispatch
	snapshot.Execution.GoalID = "goal-room"
	snapshot.Execution.GoalObjectiveRevision = 4
	actor := ActorContext{
		OwnerUserID:           snapshot.Execution.OwnerUserID,
		SessionKey:            snapshot.Execution.SessionKey,
		ExecutionID:           snapshot.Execution.ID,
		AgentID:               dispatch.TargetAgentID,
		Role:                  ExecutionActorMember,
		ActorKind:             protocol.ExecutionActorAgent,
		ScopeKind:             protocol.ExecutionScopeRoom,
		RoomID:                snapshot.Execution.RoomID,
		ConversationID:        snapshot.Execution.ConversationID,
		GoalID:                snapshot.Execution.GoalID,
		GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
		ReviewBinding: &protocol.ExecutionReviewBinding{
			ExecutionID:      dispatch.ExecutionID,
			PlanID:           dispatch.PlanID,
			WorkItemID:       dispatch.WorkItemID,
			SpecID:           dispatch.SpecID,
			AssignmentID:     dispatch.AssignmentID,
			SubmissionID:     dispatch.SubmissionID,
			ReviewDispatchID: dispatch.ID,
			TargetAgentID:    dispatch.TargetAgentID,
		},
	}

	rendered, err := NewService(&fakeRepository{snapshot: snapshot}).RuntimeContext(
		context.Background(),
		actor,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `<lane type="review" />`) ||
		!strings.Contains(rendered, `<goal id="goal-room"`) ||
		!strings.Contains(rendered, `<submission id="`+dispatch.SubmissionID+`"`) {
		t.Fatalf("Goal-bound member reviewer context = %s", rendered)
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
