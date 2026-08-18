package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestServiceEnsureMintsExecutionAndScopesCommand(t *testing.T) {
	var created orchestrationstore.CreateCommand
	repository := &fakeRepository{
		create: func(_ context.Context, command orchestrationstore.CreateCommand) (*protocol.ExecutionSnapshot, error) {
			created = command
			item := command.Execution
			item.Version = 1
			return &protocol.ExecutionSnapshot{Execution: item}, nil
		},
	}
	service := testService(repository)
	result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
		CommandID:          "tool-call-1",
		Objective:          "  ship orchestration  ",
		CompletionCriteria: []string{" tested ", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || result.ExecutionID != "execution-1" {
		t.Fatalf("result = %#v", result)
	}
	if created.Execution.ID != "execution-1" ||
		created.Execution.Objective != "ship orchestration" ||
		created.Execution.CoordinatorAgentID != "agent-lead" ||
		created.Execution.ScopeKind != protocol.ExecutionScopeDM {
		t.Fatalf("created Execution = %#v", created.Execution)
	}
	if len(created.Execution.CompletionCriteria) != 1 ||
		created.Execution.CompletionCriteria[0] != "tested" {
		t.Fatalf("completion criteria = %#v", created.Execution.CompletionCriteria)
	}
	if created.Meta.CommandID != "tool-call-1:ensure" ||
		created.Meta.EventID != "event-1" ||
		created.Meta.ActorKind != protocol.ExecutionActorAgent {
		t.Fatalf("meta = %#v", created.Meta)
	}
}

func TestServiceEnsureDoesNotSilentlyRewriteCurrentExecutionBoundary(t *testing.T) {
	for _, test := range []struct {
		name       string
		goalID     string
		reasonCode ErrorCode
		nextTool   string
	}{
		{
			name:       "transient requires explicit replacement",
			reasonCode: ErrorCodeObjectiveChangeReplace,
			nextTool:   "prepare_plan_execution",
		},
		{
			name:       "Goal-bound requires retarget",
			goalID:     "goal-1",
			reasonCode: ErrorCodeGoalRetargetRequired,
			nextTool:   "retarget_goal",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := executionSnapshot()
			snapshot.Execution.GoalID = test.goalID
			snapshot.Execution.CompletionCriteria = []string{"original criterion"}
			service := testService(&fakeRepository{snapshot: snapshot})

			result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
				CommandID:          "ensure-different-boundary",
				Objective:          "a different objective",
				CompletionCriteria: []string{"different criterion"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != MutationRejected ||
				result.ReasonCode != test.reasonCode ||
				result.Snapshot == nil ||
				result.Snapshot.Execution.ID != snapshot.Execution.ID ||
				len(result.NextActions) != 1 ||
				result.NextActions[0].Operation != test.nextTool {
				t.Fatalf("boundary-changing ensure = %#v", result)
			}
		})
	}
}

func TestServiceEnsureRejectsMissingTopLevelCompletionCriteria(t *testing.T) {
	for _, criteria := range [][]string{
		nil,
		{},
		{"", "   "},
	} {
		created := false
		service := testService(&fakeRepository{
			create: func(
				context.Context,
				orchestrationstore.CreateCommand,
			) (*protocol.ExecutionSnapshot, error) {
				created = true
				return nil, nil
			},
		})
		result, err := service.Ensure(
			context.Background(),
			coordinatorActor(),
			EnsureInput{
				CommandID:          "tool-create-without-criteria",
				Objective:          "Ship orchestration",
				CompletionCriteria: criteria,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != MutationRejected ||
			result.ReasonCode != ErrorCodeCompletionCriteriaEmpty ||
			created {
			t.Fatalf("criteria=%#v result=%#v created=%t", criteria, result, created)
		}
	}
}

func TestServiceEnsurePlanModeReturnsStructuredProposalValidation(t *testing.T) {
	service := testService(&fakeRepository{
		create: func(
			context.Context,
			orchestrationstore.CreateCommand,
		) (*protocol.ExecutionSnapshot, error) {
			t.Fatal("Plan Mode proposal must not create an Execution")
			return nil, nil
		},
	})
	actor := coordinatorActor()
	actor.PlanMode = true

	valid, err := service.Ensure(context.Background(), actor, EnsureInput{
		CommandID:          "tool-plan-proposal",
		Objective:          "Ship orchestration",
		CompletionCriteria: []string{" verified ", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if valid.Outcome != MutationNoOp ||
		valid.Snapshot != nil ||
		valid.ExecutionID != "" ||
		!strings.Contains(valid.Message, "1 top-level completion criterion") {
		t.Fatalf("valid proposal = %#v", valid)
	}

	rejected, err := service.Ensure(context.Background(), actor, EnsureInput{
		CommandID:          "tool-plan-proposal-invalid",
		Objective:          "Ship orchestration",
		CompletionCriteria: []string{"  "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Outcome != MutationRejected ||
		rejected.ReasonCode != ErrorCodeCompletionCriteriaEmpty ||
		rejected.Snapshot != nil {
		t.Fatalf("invalid proposal = %#v", rejected)
	}
}

func TestServiceProjectionCollectionLimitCoversPlanModeAndMutations(t *testing.T) {
	atLimit := makeProjectionValues(protocol.ExecutionProjectionCollectionLimit)
	overLimit := makeProjectionValues(protocol.ExecutionProjectionCollectionLimit + 1)

	t.Run("Plan Mode 32 accepted and 33 rejected", func(t *testing.T) {
		service := testService(&fakeRepository{})
		actor := coordinatorActor()
		actor.PlanMode = true
		valid, err := service.Ensure(context.Background(), actor, EnsureInput{
			CommandID:          "plan-mode-limit-valid",
			Objective:          "Validate bounded execution",
			CompletionCriteria: atLimit,
		})
		if err != nil {
			t.Fatal(err)
		}
		if valid.Outcome != MutationNoOp {
			t.Fatalf("32 criteria result = %#v", valid)
		}
		rejected, err := service.Ensure(context.Background(), actor, EnsureInput{
			CommandID:          "plan-mode-limit-overflow",
			Objective:          "Validate bounded execution",
			CompletionCriteria: overLimit,
		})
		if err != nil {
			t.Fatal(err)
		}
		if rejected.Outcome != MutationRejected ||
			rejected.ReasonCode != ErrorCodeProjectionLimitExceeded {
			t.Fatalf("33 criteria result = %#v", rejected)
		}
	})

	t.Run("Plan Mode validates Work Item collections", func(t *testing.T) {
		service := testService(&fakeRepository{})
		actor := coordinatorActor()
		actor.PlanMode = true
		draft := validPlanDraft()
		draft.Items[0].InputRefs = atLimit
		valid, err := service.PlanExecution(context.Background(), actor, PlanExecutionInput{
			CommandID:          "plan-mode-work-limit-valid",
			Objective:          "Validate bounded WorkGraph",
			CompletionCriteria: []string{"validated"},
			Draft:              draft,
		})
		if err != nil {
			t.Fatal(err)
		}
		if valid.Outcome != MutationNoOp {
			t.Fatalf("32 input refs result = %#v", valid)
		}
		draft.Items[0].InputRefs = overLimit
		rejected, err := service.PlanExecution(context.Background(), actor, PlanExecutionInput{
			CommandID:          "plan-mode-work-limit-overflow",
			Objective:          "Validate bounded WorkGraph",
			CompletionCriteria: []string{"validated"},
			Draft:              draft,
		})
		if err != nil {
			t.Fatal(err)
		}
		if rejected.Outcome != MutationRejected ||
			rejected.ReasonCode != ErrorCodeProjectionLimitExceeded {
			t.Fatalf("33 input refs result = %#v", rejected)
		}
	})

	snapshot := executionSnapshot()
	service := testService(&fakeRepository{snapshot: snapshot})
	tests := []struct {
		name string
		call func() (MutationResult, error)
	}{
		{
			name: "Submission result refs",
			call: func() (MutationResult, error) {
				return service.SubmitWork(context.Background(), coordinatorActor(), SubmitWorkInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        "submit-overflow",
					WorkItemID:       "work-1",
					ResultSummary:    "done",
					ResultRefs:       overLimit,
				})
			},
		},
		{
			name: "Submission evidence",
			call: func() (MutationResult, error) {
				return service.SubmitWork(context.Background(), coordinatorActor(), SubmitWorkInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        "submit-evidence-overflow",
					WorkItemID:       "work-1",
					ResultSummary:    "done",
					Evidence:         overLimit,
				})
			},
		},
		{
			name: "Acceptance criteria results",
			call: func() (MutationResult, error) {
				results := make([]protocol.WorkAcceptanceCriterionResult, len(overLimit))
				return service.ReviewWork(context.Background(), coordinatorActor(), ReviewWorkInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        "review-overflow",
					Decision:         protocol.WorkAcceptanceRejected,
					CriteriaResults:  results,
				})
			},
		},
		{
			name: "Acceptance criterion evidence",
			call: func() (MutationResult, error) {
				return service.ReviewWork(context.Background(), coordinatorActor(), ReviewWorkInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        "review-evidence-overflow",
					Decision:         protocol.WorkAcceptanceRejected,
					CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
						Criterion: "criterion",
						Evidence:  overLimit,
					}},
				})
			},
		},
		{
			name: "Resume evidence",
			call: func() (MutationResult, error) {
				return service.ResumeWork(context.Background(), coordinatorActor(), ResumeWorkInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        "resume-overflow",
					Resolution:       "available",
					Evidence:         overLimit,
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.call()
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != MutationRejected ||
				result.ReasonCode != ErrorCodeProjectionLimitExceeded {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func makeProjectionValues(count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = fmt.Sprintf("value-%02d", index)
	}
	return values
}

func TestServicePlanExecutionReusesStableWorkAndImmutableSpec(t *testing.T) {
	researchDraft := PlanWorkItemDraft{
		LogicalKey:         "research",
		ExistingWorkItemID: "work-research",
		Kind:               protocol.WorkItemKindProduce,
		Subject:            "Research",
		Objective:          "Collect evidence",
		Deliverable:        "Evidence set",
		AcceptanceCriteria: []string{"sources cited"},
		Required:           true,
		OutputScopes: []protocol.WorkOutputScope{{
			Scope: "dir:research",
			Mode:  protocol.WorkOutputScopeExclusive,
		}},
	}
	hash, err := workSpecHash(researchDraft)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := executionSnapshot()
	snapshot.Execution.Version = 5
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-old",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
		Version:     1,
	}
	snapshot.WorkItems = []protocol.WorkItem{{
		ID:          "work-research",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "research",
		Kind:        protocol.WorkItemKindProduce,
	}}
	snapshot.WorkItemStates = []protocol.WorkItemState{{
		WorkItemID:    "work-research",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-research",
		Status:        protocol.WorkItemStatusOpen,
		Version:       3,
	}}
	snapshot.WorkItemSpecs = []protocol.WorkItemSpec{{
		ID:          "spec-research",
		WorkItemID:  "work-research",
		ExecutionID: snapshot.Execution.ID,
		Version:     2,
		SpecHash:    hash,
	}}
	var written orchestrationstore.WritePlanCommand
	repository := &fakeRepository{
		snapshot: snapshot,
		writePlan: func(_ context.Context, command orchestrationstore.WritePlanCommand) (*protocol.ExecutionSnapshot, error) {
			written = command
			result := *snapshot
			result.Execution.Version++
			result.Plan = &command.Plan
			result.WorkItems = nil
			result.WorkItemSpecs = nil
			result.PlanItems = nil
			result.WorkItemStates = nil
			for _, work := range command.WorkItems {
				result.WorkItems = append(result.WorkItems, work.WorkItem)
				result.WorkItemSpecs = append(result.WorkItemSpecs, work.Spec)
				result.PlanItems = append(result.PlanItems, work.Item)
				result.WorkItemStates = append(result.WorkItemStates, work.State)
			}
			return &result, nil
		},
	}
	service := testService(repository)
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: 5,
		CommandID:        "tool-plan",
		Draft: PlanDraft{
			RevisionReason: "add final verification",
			Items: []PlanWorkItemDraft{
				researchDraft,
				{
					LogicalKey:         "verify",
					Kind:               protocol.WorkItemKindVerify,
					Subject:            "Verify",
					Objective:          "Check the result",
					Deliverable:        "Verification report",
					AcceptanceCriteria: []string{"all checks pass"},
					Required:           true,
					Terminal:           true,
					DependsOn: []PlanDependencyDraft{{
						LogicalKey: "research",
						Kind:       protocol.WorkDependencyHard,
					}},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || written.Meta.CommandID != "tool-plan:plan" {
		t.Fatalf("result=%#v meta=%#v", result, written.Meta)
	}
	if written.Plan.ID != "plan-1" || written.Plan.Revision != 2 ||
		written.Plan.BasePlanID != "plan-old" {
		t.Fatalf("plan = %#v", written.Plan)
	}
	if len(written.WorkItems) != 2 {
		t.Fatalf("work items = %#v", written.WorkItems)
	}
	reused := written.WorkItems[0]
	if reused.WorkItem.ID != "work-research" ||
		reused.Spec.ID != "spec-research" ||
		reused.Spec.Version != 2 ||
		reused.ExpectedStateVersion != 3 {
		t.Fatalf("reused work = %#v", reused)
	}
	if written.WorkItems[1].WorkItem.ID != "work-1" ||
		written.WorkItems[1].Spec.ID != "spec-1" {
		t.Fatalf("server minted work = %#v", written.WorkItems[1])
	}
	if len(written.Dependencies) != 1 ||
		written.Dependencies[0].WorkItemID != "work-1" ||
		written.Dependencies[0].DependsOnWorkItemID != "work-research" {
		t.Fatalf("dependencies = %#v", written.Dependencies)
	}
}

func TestServicePlanExecutionAllowsMonotonicGraphExtension(t *testing.T) {
	existing := PlanWorkItemDraft{
		LogicalKey:         "research",
		ExistingWorkItemID: "work-research",
		Kind:               protocol.WorkItemKindProduce,
		Subject:            "Research",
		Objective:          "Collect evidence",
		Deliverable:        "Evidence set",
		AcceptanceCriteria: []string{"sources cited"},
		Required:           true,
		OutputScopes: []protocol.WorkOutputScope{{
			Scope: "dir:research",
			Mode:  protocol.WorkOutputScopeExclusive,
		}},
	}
	hash, err := workSpecHash(existing)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := executionSnapshot()
	snapshot.Execution.Version = 5
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-old",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
		Version:     1,
	}
	snapshot.WorkItems = []protocol.WorkItem{{
		ID:          "work-research",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "research",
		Kind:        protocol.WorkItemKindProduce,
	}}
	snapshot.WorkItemStates = []protocol.WorkItemState{{
		WorkItemID:    "work-research",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-research",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	}}
	snapshot.WorkItemSpecs = []protocol.WorkItemSpec{{
		ID:          "spec-research",
		WorkItemID:  "work-research",
		ExecutionID: snapshot.Execution.ID,
		Version:     1,
		SpecHash:    hash,
	}}
	snapshot.PlanItems = []protocol.ExecutionPlanItem{{
		PlanID:      "plan-old",
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-research",
		SpecID:      "spec-research",
		Required:    true,
	}}
	var written orchestrationstore.WritePlanCommand
	writeCalls := 0
	service := testService(&fakeRepository{
		snapshot: snapshot,
		writePlan: func(
			_ context.Context,
			command orchestrationstore.WritePlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			writeCalls++
			written = command
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			result.Plan = &command.Plan
			return result, nil
		},
	})
	result, err := service.PlanExecution(
		context.Background(),
		coordinatorActor(),
		PlanExecutionInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "append-verify",
			Draft: PlanDraft{
				RevisionReason: "append final verification",
				Items: []PlanWorkItemDraft{
					existing,
					{
						LogicalKey:         "verify",
						Kind:               protocol.WorkItemKindVerify,
						Subject:            "Verify",
						Objective:          "Verify accepted evidence",
						Deliverable:        "Verification report",
						AcceptanceCriteria: []string{"all checks pass"},
						Required:           true,
						Terminal:           true,
						DependsOn: []PlanDependencyDraft{{
							LogicalKey: "research",
							Kind:       protocol.WorkDependencyHard,
						}},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		written.SupersedeActiveWork ||
		len(written.WorkItems) != 2 ||
		written.WorkItems[0].WorkItem.ID != "work-research" ||
		writeCalls != 1 {
		t.Fatalf("result=%#v command=%#v", result, written)
	}

	existingWithNewIncomingEdge := existing
	existingWithNewIncomingEdge.DependsOn = []PlanDependencyDraft{{
		LogicalKey: "prerequisite",
		Kind:       protocol.WorkDependencyHard,
	}}
	result, err = service.PlanExecution(
		context.Background(),
		coordinatorActor(),
		PlanExecutionInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "backfill-old-readiness",
			Draft: PlanDraft{
				RevisionReason: "make existing research depend on a new prerequisite",
				Items: []PlanWorkItemDraft{
					{
						LogicalKey:         "prerequisite",
						Kind:               protocol.WorkItemKindProduce,
						Subject:            "Prerequisite",
						Objective:          "Produce an additional prerequisite",
						Deliverable:        "Prerequisite evidence",
						AcceptanceCriteria: []string{"evidence exists"},
						Required:           true,
						OutputScopes: []protocol.WorkOutputScope{{
							Scope: "dir:prerequisite",
							Mode:  protocol.WorkOutputScopeExclusive,
						}},
					},
					existingWithNewIncomingEdge,
					{
						LogicalKey:         "verify",
						Kind:               protocol.WorkItemKindVerify,
						Subject:            "Verify",
						Objective:          "Verify accepted evidence",
						Deliverable:        "Verification report",
						AcceptanceCriteria: []string{"all checks pass"},
						Required:           true,
						Terminal:           true,
						DependsOn: []PlanDependencyDraft{{
							LogicalKey: "research",
							Kind:       protocol.WorkDependencyHard,
						}},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeCompletionBlocked ||
		!strings.Contains(result.Message, "supersede_active_work=true") ||
		writeCalls != 1 {
		t.Fatalf(
			"incoming edge to existing node result=%#v writeCalls=%d",
			result,
			writeCalls,
		)
	}
}

func TestServicePlanExecutionRequiresExplicitSupersedeForNodeRemoval(t *testing.T) {
	existing := PlanWorkItemDraft{
		LogicalKey:         "research",
		ExistingWorkItemID: "work-research",
		Kind:               protocol.WorkItemKindProduce,
		Subject:            "Research",
		Objective:          "Collect evidence",
		Deliverable:        "Evidence set",
		AcceptanceCriteria: []string{"sources cited"},
		Required:           true,
		OutputScopes: []protocol.WorkOutputScope{{
			Scope: "dir:research",
			Mode:  protocol.WorkOutputScopeExclusive,
		}},
	}
	hash, err := workSpecHash(existing)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := executionSnapshot()
	snapshot.Execution.Version = 5
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-old",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
		Version:     1,
	}
	snapshot.WorkItems = []protocol.WorkItem{{
		ID:          "work-research",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "research",
		Kind:        protocol.WorkItemKindProduce,
	}}
	snapshot.WorkItemStates = []protocol.WorkItemState{{
		WorkItemID:    "work-research",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-research",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	}}
	snapshot.WorkItemSpecs = []protocol.WorkItemSpec{{
		ID:          "spec-research",
		WorkItemID:  "work-research",
		ExecutionID: snapshot.Execution.ID,
		Version:     1,
		SpecHash:    hash,
	}}
	snapshot.PlanItems = []protocol.ExecutionPlanItem{{
		PlanID:      "plan-old",
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-research",
		SpecID:      "spec-research",
		Required:    true,
	}}
	written := false
	service := testService(&fakeRepository{
		snapshot: snapshot,
		writePlan: func(
			context.Context,
			orchestrationstore.WritePlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			written = true
			return nil, nil
		},
	})
	result, err := service.PlanExecution(
		context.Background(),
		coordinatorActor(),
		PlanExecutionInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "omit-existing-node",
			Draft: PlanDraft{
				RevisionReason: "replace research",
				Items: []PlanWorkItemDraft{{
					LogicalKey:         "verify",
					Kind:               protocol.WorkItemKindVerify,
					Subject:            "Verify",
					Objective:          "Verify replacement",
					Deliverable:        "Verification report",
					AcceptanceCriteria: []string{"all checks pass"},
					Required:           true,
					Terminal:           true,
				}},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeCompletionBlocked ||
		!strings.Contains(result.Message, "supersede_active_work=true") ||
		written {
		t.Fatalf("result=%#v written=%t", result, written)
	}
}

func TestServiceSubmitWorkTransparentlyRunsAttempt(t *testing.T) {
	initial := assignedExecutionSnapshot()
	var calls []string
	repository := &fakeRepository{
		snapshot: initial,
		startAttempt: func(_ context.Context, command orchestrationstore.StartAttemptCommand) (*protocol.ExecutionSnapshot, error) {
			calls = append(calls, command.Meta.CommandID)
			if command.ExpectedExecutionVersion != 10 ||
				command.ExpectedAssignmentVersion != 1 ||
				command.ExpectedAttemptVersion != 1 {
				t.Fatalf("start command = %#v", command)
			}
			result := cloneExecutionSnapshot(initial)
			result.Execution.Version = 11
			result.Assignments[0].Status = protocol.WorkAssignmentStatusActive
			result.Assignments[0].Version = 2
			result.Attempts[0].Status = protocol.WorkAttemptStatusRunning
			result.Attempts[0].Version = 2
			result.Attempts[0].RuntimeRoundID = command.Attempt.RuntimeRoundID
			return result, nil
		},
		finishAttempt: func(_ context.Context, command orchestrationstore.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
			calls = append(calls, command.Meta.CommandID)
			if command.ExpectedExecutionVersion != 11 ||
				command.ExpectedAttemptVersion != 2 ||
				command.Attempt.Status != protocol.WorkAttemptStatusSucceeded {
				t.Fatalf("finish command = %#v", command)
			}
			result := cloneExecutionSnapshot(initial)
			result.Execution.Version = 12
			result.Assignments[0].Status = protocol.WorkAssignmentStatusActive
			result.Assignments[0].Version = 2
			result.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
			result.Attempts[0].Version = 3
			return result, nil
		},
		submit: func(_ context.Context, command orchestrationstore.SubmitCommand) (*protocol.ExecutionSnapshot, error) {
			calls = append(calls, command.Meta.CommandID)
			if command.ExpectedExecutionVersion != 12 ||
				command.ExpectedAssignmentVersion != 2 ||
				command.Submission.AttemptID != "attempt-1" ||
				command.Submission.SubmitterAgentID != "agent-worker" {
				t.Fatalf("submit command = %#v", command)
			}
			result := cloneExecutionSnapshot(initial)
			result.Execution.Version = 13
			result.Assignments[0].Status = protocol.WorkAssignmentStatusActive
			result.Assignments[0].Version = 3
			result.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
			result.Attempts[0].Version = 3
			result.Submissions = []protocol.WorkSubmission{command.Submission}
			return result, nil
		},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.AgentID = "agent-worker"
	actor.Role = ExecutionActorMember
	actor.RuntimeRoundID = "runtime-round-1"
	result, err := service.SubmitWork(context.Background(), actor, SubmitWorkInput{
		ExecutionID:      initial.Execution.ID,
		SnapshotRevision: 10,
		CommandID:        "tool-submit",
		LogicalKey:       "research",
		ResultSummary:    "Research completed",
		Evidence:         []string{"go test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || result.SnapshotRevision != 13 {
		t.Fatalf("result = %#v", result)
	}
	wantCalls := []string{
		"tool-submit:submit-start",
		"tool-submit:submit-finish",
		"tool-submit:submit-record",
	}
	if strings.Join(calls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestReviewDispatchExistsOnlyForCrossAgentReturn(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.ScopeKind = protocol.ExecutionScopeRoom
	snapshot.Execution.CoordinatorAgentID = "agent-lead"
	assignment := snapshot.Assignments[0]
	assignment.OwnerAgentID = "agent-worker"

	assignment.ReturnToAgentID = "agent-worker"
	if needsReviewDispatch(snapshot, &assignment) {
		t.Fatal("self-review must stay in the current Agent responsibility")
	}

	assignment.ReturnToAgentID = "agent-reviewer"
	if !needsReviewDispatch(snapshot, &assignment) {
		t.Fatal("cross-Agent review must create a durable Room return")
	}
	instruction := reviewDispatchInstruction(
		snapshot,
		&assignment,
		protocol.WorkSubmission{
			ID:            "submission-1",
			ResultSummary: "evidence ready",
		},
		protocol.WorkItem{LogicalKey: "research"},
	)
	for _, expected := range []string{
		"coordinator agent-lead",
		"Room communication",
		"do not send a status-only handoff",
		"do not wait for a user continuation message",
	} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("cross-Agent review instruction missing %q: %s", expected, instruction)
		}
	}

	snapshot.Execution.ScopeKind = protocol.ExecutionScopeDM
	if needsReviewDispatch(snapshot, &assignment) {
		t.Fatal("DM review does not need a Room return outbox")
	}
}

func TestServiceSubmitWorkRequiresResumeBeforeStartingAttempt(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.WorkItemStates[0].Status = protocol.WorkItemStatusWaitingInput
	snapshot.WorkItemStates[0].BlockReason = "approval missing"
	snapshot.WorkItemStates[0].NeededInput = "approval-42"
	service := testService(&fakeRepository{snapshot: snapshot})
	actor := coordinatorActor()
	actor.AgentID = "agent-worker"
	actor.Role = ExecutionActorMember

	result, err := service.SubmitWork(context.Background(), actor, SubmitWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-submit-blocked",
		LogicalKey:       "research",
		ResultSummary:    "Research completed",
		Evidence:         []string{"go test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeCompletionBlocked {
		t.Fatalf("blocked SubmitWork = %#v", result)
	}
}

func TestServiceAllowsSelectedRoomAgentToReviewOwnSubmission(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.ScopeKind = protocol.ExecutionScopeRoom
	snapshot.Execution.RoomID = "room-1"
	snapshot.Execution.ConversationID = "conversation-1"
	snapshot.Execution.CoordinatorAgentID = "agent-worker"
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].ReturnToAgentID = "agent-worker"
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		AttemptID:        "attempt-1",
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "Research completed",
	}}
	snapshot.ReviewDispatches = []protocol.ExecutionReviewDispatch{{
		ID:            "review-dispatch-1",
		ExecutionID:   snapshot.Execution.ID,
		PlanID:        "plan-1",
		WorkItemID:    "work-1",
		SpecID:        "spec-1",
		AssignmentID:  "assignment-1",
		SubmissionID:  "submission-1",
		TargetAgentID: "agent-worker",
		Status:        protocol.ExecutionReviewDispatchStatusDelivered,
	}}
	snapshot.CompletionBlockers = []string{"another required Work Item remains"}
	repository := &fakeRepository{
		snapshot: snapshot,
		review: func(_ context.Context, command orchestrationstore.ReviewCommand) (*protocol.ExecutionSnapshot, error) {
			if command.Acceptance.ReviewerID != "agent-worker" ||
				command.Acceptance.SubmissionID != "submission-1" {
				t.Fatalf("self review command = %+v", command)
			}
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			result.Acceptances = append(result.Acceptances, command.Acceptance)
			return result, nil
		},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.AgentID = "agent-worker"
	actor.ScopeKind = protocol.ExecutionScopeRoom
	actor.RoomID = "room-1"
	actor.ConversationID = "conversation-1"
	actor.ReviewBinding = &protocol.ExecutionReviewBinding{
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "agent-worker",
	}

	result, err := service.ReviewWork(context.Background(), actor, ReviewWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-review-own",
		SubmissionID:     "submission-1",
		Decision:         protocol.WorkAcceptanceAccepted,
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "sources cited",
			Passed:    true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied {
		t.Fatalf("result = %#v", result)
	}
}

func TestServiceRequiresReviewBindingForRoomCoordinator(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.ScopeKind = protocol.ExecutionScopeRoom
	snapshot.Execution.RoomID = "room-1"
	snapshot.Execution.ConversationID = "conversation-1"
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].ReturnToAgentID = snapshot.Execution.CoordinatorAgentID
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		AttemptID:        "attempt-1",
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "Research completed",
	}}
	service := testService(&fakeRepository{snapshot: snapshot})
	actor := coordinatorActor()
	actor.ScopeKind = protocol.ExecutionScopeRoom
	actor.RoomID = "room-1"
	actor.ConversationID = "conversation-1"

	result, err := service.ReviewWork(context.Background(), actor, ReviewWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-review-unbound",
		SubmissionID:     "submission-1",
		Decision:         protocol.WorkAcceptanceAccepted,
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "sources cited",
			Passed:    true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeConversationOnly {
		t.Fatalf("unbound Room review result = %#v", result)
	}
}

func TestServiceRejectsCoordinatorMutationFromMember(t *testing.T) {
	repository := &fakeRepository{snapshot: executionSnapshot()}
	service := testService(repository)
	actor := coordinatorActor()
	actor.AgentID = "agent-member"
	actor.Role = ExecutionActorMember
	result, err := service.CompleteIfReady(context.Background(), actor, CompleteExecutionInput{
		ExecutionID:      repository.snapshot.Execution.ID,
		SnapshotRevision: repository.snapshot.Execution.Version,
		CommandID:        "tool-complete",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected || result.ReasonCode != ErrorCodeWrongOwner {
		t.Fatalf("result = %#v", result)
	}
}

func TestServicePlanExecutionSupersedesActiveWorkOnlyWithExplicitReasonedOptIn(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	var written orchestrationstore.WritePlanCommand
	repository := &fakeRepository{
		snapshot: snapshot,
		writePlan: func(
			_ context.Context,
			command orchestrationstore.WritePlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			written = command
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			result.Plan = &command.Plan
			return result, nil
		},
	}
	service := testService(repository)
	draft := PlanDraft{
		RevisionReason: "replace obsolete scope",
		Items: []PlanWorkItemDraft{{
			LogicalKey:         "verify_new_scope",
			Kind:               protocol.WorkItemKindVerify,
			Subject:            "Verify new scope",
			Objective:          "Verify replacement result",
			Deliverable:        "Replacement verification",
			AcceptanceCriteria: []string{"new scope verified"},
			Required:           true,
			Terminal:           true,
		}},
	}
	rejected, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-plan-default",
		Draft:            draft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Outcome != MutationRejected ||
		rejected.ReasonCode != ErrorCodeCompletionBlocked ||
		written.Plan.ID != "" {
		t.Fatalf("default replacement = %#v written=%#v", rejected, written)
	}

	missingReason := draft
	missingReason.RevisionReason = ""
	rejected, err = service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:         snapshot.Execution.ID,
		SnapshotRevision:    snapshot.Execution.Version,
		CommandID:           "tool-plan-no-reason",
		SupersedeActiveWork: true,
		Draft:               missingReason,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Outcome != MutationRejected ||
		rejected.ReasonCode != ErrorCodeInvalidInput ||
		written.Plan.ID != "" {
		t.Fatalf("reasonless replacement = %#v written=%#v", rejected, written)
	}

	applied, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:         snapshot.Execution.ID,
		SnapshotRevision:    snapshot.Execution.Version,
		CommandID:           "tool-plan-opt-in",
		SupersedeActiveWork: true,
		Draft:               draft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Outcome != MutationApplied ||
		!written.SupersedeActiveWork ||
		written.Plan.RevisionReason != "replace obsolete scope" ||
		written.Meta.CommandID != "tool-plan-opt-in:plan" {
		t.Fatalf("opt-in replacement result=%#v command=%#v", applied, written)
	}
}

func TestServicePlanExecutionEquivalentNormalizedDraftIsSemanticNoOp(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.WorkItems[0].Kind = protocol.WorkItemKindVerify
	snapshot.PlanItems[0].Terminal = true
	draft := PlanDraft{
		RevisionReason: "retry after context refresh",
		Items: []PlanWorkItemDraft{{
			LogicalKey:         "research",
			ExistingWorkItemID: "work-1",
			Kind:               protocol.WorkItemKindVerify,
			Subject:            "Research",
			Objective:          "Collect evidence",
			Deliverable:        "Evidence set",
			AcceptanceCriteria: []string{"sources cited"},
			Required:           true,
			Terminal:           true,
		}},
	}
	hash, err := workSpecHash(draft.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	snapshot.WorkItemSpecs[0].SpecHash = hash
	written := false
	service := testService(&fakeRepository{
		snapshot: snapshot,
		writePlan: func(
			context.Context,
			orchestrationstore.WritePlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			written = true
			return nil, nil
		},
	})
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version - 1,
		CommandID:        "new-tool-use-id",
		Draft:            draft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationNoOp ||
		result.SnapshotRevision != snapshot.Execution.Version ||
		written {
		t.Fatalf("equivalent Plan result=%#v written=%t", result, written)
	}
	outsider := coordinatorActor()
	outsider.AgentID = "agent-outsider"
	outsider.Role = ExecutionActorMember
	result, err = service.PlanExecution(context.Background(), outsider, PlanExecutionInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version - 1,
		CommandID:        "outsider-tool-use-id",
		Draft:            draft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeWrongOwner ||
		written {
		t.Fatalf("outsider equivalent Plan result=%#v written=%t", result, written)
	}

	changed := draft
	changed.Items = append([]PlanWorkItemDraft(nil), draft.Items...)
	changed.Items[0].Deliverable = "Different evidence package"
	result, err = service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "changed-tool-use-id",
		Draft:            changed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeCompletionBlocked ||
		written {
		t.Fatalf("changed Plan result=%#v written=%t", result, written)
	}
}

func TestServicePlanExecutionNeverSupersedesUnreviewedSubmission(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-pending",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           snapshot.Plan.ID,
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		AttemptID:        "attempt-1",
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "pending review",
	}}
	written := false
	repository := &fakeRepository{
		snapshot: snapshot,
		writePlan: func(
			context.Context,
			orchestrationstore.WritePlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			written = true
			return nil, nil
		},
	}
	service := testService(repository)
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:         snapshot.Execution.ID,
		SnapshotRevision:    snapshot.Execution.Version,
		CommandID:           "tool-plan-unreviewed",
		SupersedeActiveWork: true,
		Draft: PlanDraft{
			RevisionReason: "replace pending result",
			Items: []PlanWorkItemDraft{{
				LogicalKey:         "verify",
				Kind:               protocol.WorkItemKindVerify,
				Subject:            "Verify",
				Objective:          "Verify replacement",
				Deliverable:        "Verification",
				AcceptanceCriteria: []string{"verified"},
				Required:           true,
				Terminal:           true,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeCompletionBlocked ||
		written {
		t.Fatalf("unreviewed replacement result=%#v written=%t", result, written)
	}
}

func TestServiceResumeWorkRequiresEvidenceAndAuthorizedOwner(t *testing.T) {
	for _, test := range []struct {
		name               string
		actor              ActorContext
		releasedAssignment bool
	}{
		{
			name: "Assignment owner",
			actor: func() ActorContext {
				actor := coordinatorActor()
				actor.AgentID = "agent-worker"
				actor.Role = ExecutionActorMember
				return actor
			}(),
		},
		{
			name: "released Assignment owner",
			actor: func() ActorContext {
				actor := coordinatorActor()
				actor.AgentID = "agent-worker"
				actor.Role = ExecutionActorMember
				return actor
			}(),
			releasedAssignment: true,
		},
		{name: "coordinator", actor: coordinatorActor()},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := assignedExecutionSnapshot()
			snapshot.WorkItemStates[0].Status = protocol.WorkItemStatusWaitingInput
			snapshot.WorkItemStates[0].BlockReason = "approval missing"
			snapshot.WorkItemStates[0].NeededInput = "approval"
			if test.releasedAssignment {
				snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusReleased
			}
			var resumed orchestrationstore.ResumeCommand
			repository := &fakeRepository{
				snapshot: snapshot,
				resume: func(
					_ context.Context,
					command orchestrationstore.ResumeCommand,
				) (*protocol.ExecutionSnapshot, error) {
					resumed = command
					result := cloneExecutionSnapshot(snapshot)
					result.Execution.Version++
					result.WorkItemStates[0] = command.State
					result.WorkItemStates[0].Version++
					return result, nil
				},
			}
			service := testService(repository)
			result, err := service.ResumeWork(context.Background(), test.actor, ResumeWorkInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        "tool-resume",
				LogicalKey:       "research",
				Resolution:       " approval granted ",
				Evidence:         []string{" approval-42 ", ""},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != MutationApplied ||
				resumed.ExpectedStateVersion != 1 ||
				resumed.Resolution != "approval granted" ||
				len(resumed.Evidence) != 1 ||
				resumed.Evidence[0] != "approval-42" ||
				resumed.State.Status != protocol.WorkItemStatusOpen ||
				resumed.Meta.CommandID != "tool-resume:resume" {
				t.Fatalf("resume result=%#v command=%#v", result, resumed)
			}
		})
	}

	snapshot := assignedExecutionSnapshot()
	snapshot.WorkItemStates[0].Status = protocol.WorkItemStatusWaitingInput
	snapshot.WorkItemStates[0].BlockReason = "approval missing"
	snapshot.WorkItemStates[0].NeededInput = "approval"
	service := testService(&fakeRepository{snapshot: snapshot})
	outsider := coordinatorActor()
	outsider.AgentID = "agent-outsider"
	outsider.Role = ExecutionActorMember
	rejected, err := service.ResumeWork(context.Background(), outsider, ResumeWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-resume-outsider",
		LogicalKey:       "research",
		Resolution:       "approval granted",
		Evidence:         []string{"approval-42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Outcome != MutationRejected || rejected.ReasonCode != ErrorCodeWrongOwner {
		t.Fatalf("outsider resume = %#v", rejected)
	}
	missingEvidence, err := service.ResumeWork(context.Background(), coordinatorActor(), ResumeWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-resume-no-evidence",
		LogicalKey:       "research",
		Resolution:       "approval granted",
	})
	if err != nil {
		t.Fatal(err)
	}
	if missingEvidence.Outcome != MutationRejected ||
		missingEvidence.ReasonCode != ErrorCodeInvalidInput {
		t.Fatalf("evidenceless resume = %#v", missingEvidence)
	}

	alreadyOpen := assignedExecutionSnapshot()
	alreadyOpen.Execution.Version++
	service = testService(&fakeRepository{snapshot: alreadyOpen})
	replayed, err := service.ResumeWork(context.Background(), coordinatorActor(), ResumeWorkInput{
		ExecutionID:      alreadyOpen.Execution.ID,
		SnapshotRevision: alreadyOpen.Execution.Version - 1,
		CommandID:        "tool-resume-replay",
		LogicalKey:       "research",
		Resolution:       "approval granted",
		Evidence:         []string{"approval-42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Outcome != MutationNoOp ||
		replayed.SnapshotRevision != alreadyOpen.Execution.Version {
		t.Fatalf("semantic Resume replay = %#v", replayed)
	}
}

func TestServicePlanModeValidatesProposalWithoutWritingAndRuntimeContextMatches(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	var written orchestrationstore.WritePlanCommand
	repository := &fakeRepository{
		snapshot: snapshot,
		writePlan: func(
			_ context.Context,
			command orchestrationstore.WritePlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			written = command
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			return result, nil
		},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.PlanMode = true
	result, err := service.PlanExecution(context.Background(), actor, PlanExecutionInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-propose",
		Draft: PlanDraft{Items: []PlanWorkItemDraft{{
			LogicalKey:         "verify",
			Kind:               protocol.WorkItemKindVerify,
			Subject:            "Verify",
			Objective:          "Verify result",
			Deliverable:        "Verification",
			AcceptanceCriteria: []string{"verified"},
			Required:           true,
			Terminal:           true,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationNoOp || written.Plan.ID != "" {
		t.Fatalf("result=%#v plan=%#v", result, written.Plan)
	}
	for _, action := range result.NextActions {
		if action.Operation == "assign_work" {
			t.Fatalf("Plan Mode exposed assign_work: %#v", result.NextActions)
		}
	}
	contextValue, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	allowedEnd := strings.Index(contextValue, "</allowed_actions>")
	if allowedEnd < 0 {
		t.Fatalf("runtime context has no allowed_actions = %s", contextValue)
	}
	allowed := contextValue[:allowedEnd]
	if !strings.Contains(contextValue, "<mode plan_only=") ||
		!strings.Contains(contextValue, "true") ||
		!strings.Contains(allowed, "<action>prepare_plan_execution</action>") ||
		strings.Contains(allowed, "<action>plan_execution</action>") ||
		strings.Contains(allowed, "<action>assign_work</action>") {
		t.Fatalf("runtime context = %s", contextValue)
	}
}

func TestServicePlanModeValidatesProposalWithoutCreatingExecution(t *testing.T) {
	repository := &fakeRepository{}
	service := testService(repository)
	actor := coordinatorActor()
	actor.PlanMode = true

	result, err := service.PlanExecution(context.Background(), actor, PlanExecutionInput{
		CommandID:          "tool-plan-proposal",
		Objective:          "Deliver a verified proposal",
		CompletionCriteria: []string{"The proposal is accepted"},
		Draft: PlanDraft{Items: []PlanWorkItemDraft{{
			LogicalKey:         "verify",
			Kind:               protocol.WorkItemKindVerify,
			Subject:            "Verify",
			Objective:          "Verify result",
			Deliverable:        "Verification report",
			AcceptanceCriteria: []string{"verified"},
			Required:           true,
			Terminal:           true,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationNoOp || result.ExecutionID != "" || result.Snapshot != nil {
		t.Fatalf("result = %#v", result)
	}
	if len(result.NextActions) != 1 || result.NextActions[0].Operation != "prepare_plan_execution" {
		t.Fatalf("next actions = %#v", result.NextActions)
	}
}

func TestServiceRuntimeContextExposesUnmanagedBoundary(t *testing.T) {
	service := testService(&fakeRepository{})
	actor := coordinatorActor()
	contextValue, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contextValue, `<execution state="unmanaged" />`) ||
		!strings.Contains(contextValue, `role="coordinator"`) ||
		!strings.Contains(contextValue, `<action>plan_execution</action>`) {
		t.Fatalf("runtime context = %s", contextValue)
	}
}

func TestServiceRuntimeContextPublishesAuthoritativeGoalPromotionReason(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.RootRoundID = "round-before-boundary"
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	service := testService(&fakeRepository{snapshot: snapshot})
	service.SetGoalPromotionGateway(goalPromotionGatewayWithAvailability{
		GoalPromotionGateway: goalPromotionGatewayFunc(func(
			context.Context,
			GoalPromotionRequest,
		) (GoalPromotionBinding, error) {
			return GoalPromotionBinding{}, nil
		}),
	})
	actor := coordinatorActor()
	actor.RootRoundID = "round-after-boundary"

	contextValue, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contextValue, `<goal_promotion eligible="true">`) ||
		!strings.Contains(contextValue, `<activation_reason>observed_boundary</activation_reason>`) {
		t.Fatalf("runtime context = %s", contextValue)
	}
}

func TestServiceRuntimeContextFailsClosedWhenGoalPolicyAvailabilityIsUnknown(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.RootRoundID = "round-before-boundary"
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	service := testService(&fakeRepository{snapshot: snapshot})
	actor := coordinatorActor()
	actor.RootRoundID = "round-after-boundary"

	contextValue, err := service.RuntimeContext(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	allowedEnd := strings.Index(contextValue, "</allowed_actions>")
	if allowedEnd < 0 ||
		!strings.Contains(contextValue, "<blocker>goal_policy_unavailable</blocker>") ||
		strings.Contains(
			contextValue[:allowedEnd],
			"<action>promote_execution_to_goal</action>",
		) {
		t.Fatalf("runtime context = %s", contextValue)
	}
}

func TestServicePromotionReusesGatewayGoalAfterBindConflict(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	snapshot.Execution.RootRoundID = "round-original"
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-1",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
		Version:     1,
	}
	snapshot.PlanItems = []protocol.ExecutionPlanItem{{
		PlanID:      "plan-1",
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-1",
		SpecID:      "spec-1",
		Required:    true,
		Terminal:    true,
	}}
	var gatewayCommands []string
	gateway := goalPromotionGatewayFunc(func(
		_ context.Context,
		request GoalPromotionRequest,
	) (GoalPromotionBinding, error) {
		gatewayCommands = append(gatewayCommands, request.CommandID)
		return GoalPromotionBinding{
			GoalID:                "goal-stable",
			GoalObjectiveRevision: 1,
			ActivationOrigin:      protocol.GoalActivationOriginAdaptivePromoted,
			ActivationReason:      protocol.GoalActivationReasonSubstantialComplexity,
		}, nil
	})
	bindCalls := 0
	repository := &fakeRepository{
		snapshot: snapshot,
		bindGoal: func(
			_ context.Context,
			command orchestrationstore.BindGoalCommand,
		) (*protocol.ExecutionSnapshot, error) {
			bindCalls++
			if command.Execution.GoalID != "goal-stable" ||
				command.Meta.CommandID != "tool-promote:bind-goal" {
				t.Fatalf("bind command = %#v", command)
			}
			if bindCalls == 1 {
				return nil, orchestrationstore.ErrVersionConflict
			}
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			result.Execution.GoalID = command.Execution.GoalID
			result.Execution.GoalObjectiveRevision = command.Execution.GoalObjectiveRevision
			result.Execution.GoalActivationOrigin = command.Execution.GoalActivationOrigin
			result.Execution.GoalActivationReason = command.Execution.GoalActivationReason
			return result, nil
		},
	}
	service := testService(repository)
	service.SetGoalPromotionGateway(gateway)
	confirmation := &confirmingGoalBindingGateway{}
	service.SetExplicitGoalBindingGateway(confirmation)
	actor := coordinatorActor()
	actor.RootRoundID = "round-after-boundary"
	input := PromoteExecutionToGoalInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-promote",
		ActivationReason: protocol.GoalActivationReasonObservedBoundary,
	}
	first, err := service.PromoteExecutionToGoal(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Outcome != MutationRejected ||
		first.ReasonCode != ErrorCodeStaleExecution ||
		!strings.Contains(first.Message, "retry with the same semantic arguments") {
		t.Fatalf("first result = %#v", first)
	}
	second, err := service.PromoteExecutionToGoal(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	if second.Outcome != MutationApplied || second.Snapshot.Execution.GoalID != "goal-stable" {
		t.Fatalf("second result = %#v", second)
	}
	if len(gatewayCommands) != 2 ||
		gatewayCommands[0] != "tool-promote:promote-goal" ||
		gatewayCommands[1] != gatewayCommands[0] {
		t.Fatalf("gateway commands = %#v", gatewayCommands)
	}
	if confirmation.confirmCalls != 1 ||
		confirmation.lastConfirmation.ExecutionID != snapshot.Execution.ID ||
		confirmation.lastConfirmation.GoalID != "goal-stable" {
		t.Fatalf("Goal confirmation = %#v", confirmation)
	}
}

func TestServicePromotionAllowsAgentChoiceWithoutSuggestedSignal(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-1",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
	}
	for index := 0; index < 8; index++ {
		snapshot.PlanItems = append(snapshot.PlanItems, protocol.ExecutionPlanItem{
			PlanID:      "plan-1",
			ExecutionID: snapshot.Execution.ID,
			WorkItemID:  "work-" + string(rune('a'+index)),
			SpecID:      "spec-" + string(rune('a'+index)),
			Required:    true,
		})
	}
	gatewayCalled := false
	repository := &fakeRepository{
		snapshot: snapshot,
		bindGoal: func(
			_ context.Context,
			command orchestrationstore.BindGoalCommand,
		) (*protocol.ExecutionSnapshot, error) {
			updated := cloneExecutionSnapshot(snapshot)
			updated.Execution.Version++
			updated.Execution.GoalID = command.Execution.GoalID
			updated.Execution.GoalObjectiveRevision = command.Execution.GoalObjectiveRevision
			updated.Execution.GoalActivationOrigin = command.Execution.GoalActivationOrigin
			updated.Execution.GoalActivationReason = command.Execution.GoalActivationReason
			return updated, nil
		},
	}
	service := testService(repository)
	confirmation := &confirmingGoalBindingGateway{}
	service.SetExplicitGoalBindingGateway(confirmation)
	service.SetGoalPromotionGateway(goalPromotionGatewayFunc(func(
		context.Context,
		GoalPromotionRequest,
	) (GoalPromotionBinding, error) {
		gatewayCalled = true
		return GoalPromotionBinding{
			GoalID:                "goal-agent-choice",
			GoalObjectiveRevision: 1,
			ActivationOrigin:      protocol.GoalActivationOriginAdaptivePromoted,
			ActivationReason:      protocol.GoalActivationReasonObservedBoundary,
		}, nil
	}))
	result, err := service.PromoteExecutionToGoal(
		context.Background(),
		coordinatorActor(),
		PromoteExecutionToGoalInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "tool-complex",
			ActivationReason: protocol.GoalActivationReasonSubstantialComplexity,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		result.Snapshot == nil ||
		result.Snapshot.Execution.GoalID != "goal-agent-choice" ||
		!gatewayCalled {
		t.Fatalf("result=%#v gatewayCalled=%t", result, gatewayCalled)
	}
	if confirmation.confirmCalls != 1 ||
		confirmation.lastConfirmation.GoalID != "goal-agent-choice" {
		t.Fatalf("Goal confirmation = %#v", confirmation)
	}
}

func TestServiceAcceptedTerminalReviewCompletesExecutionAutomatically(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.Version = 13
	snapshot.PlanItems[0].Terminal = true
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].Version = 3
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
	snapshot.Attempts[0].Version = 3
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		AttemptID:        "attempt-1",
		Sequence:         1,
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "done",
	}}
	var calls []string
	repository := &fakeRepository{
		snapshot: snapshot,
		review: func(
			_ context.Context,
			command orchestrationstore.ReviewCommand,
		) (*protocol.ExecutionSnapshot, error) {
			calls = append(calls, command.Meta.CommandID)
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version = 14
			result.Assignments[0].Status = protocol.WorkAssignmentStatusCompleted
			result.Assignments[0].Version = 4
			result.Acceptances = []protocol.WorkAcceptance{command.Acceptance}
			result.CompletionBlockers = nil
			return result, nil
		},
		complete: func(
			_ context.Context,
			command orchestrationstore.CompleteCommand,
		) (*protocol.ExecutionSnapshot, error) {
			calls = append(calls, command.Meta.CommandID)
			if command.ExpectedExecutionVersion != 14 {
				t.Fatalf("complete command = %#v", command)
			}
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version = 15
			result.Execution.Status = protocol.ExecutionStatusCompleted
			result.Assignments[0].Status = protocol.WorkAssignmentStatusCompleted
			result.Acceptances = []protocol.WorkAcceptance{{Decision: protocol.WorkAcceptanceAccepted}}
			result.CompletionBlockers = nil
			return result, nil
		},
	}
	service := testService(repository)
	result, err := service.ReviewWork(context.Background(), coordinatorActor(), ReviewWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: 13,
		CommandID:        "tool-review",
		SubmissionID:     "submission-1",
		Decision:         protocol.WorkAcceptanceAccepted,
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "sources cited",
			Passed:    true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		result.Snapshot.Execution.Status != protocol.ExecutionStatusCompleted ||
		strings.Join(calls, ",") != "tool-review:review,tool-review:complete-after-review" {
		t.Fatalf("result=%#v calls=%#v", result, calls)
	}
	for _, action := range result.NextActions {
		if action.Operation == "complete_execution" {
			t.Fatalf("model-facing completion action leaked: %#v", result.NextActions)
		}
	}
}

func TestServicePendingSubmissionRejectsBlockAndTakeoverButRemainsReviewable(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.Version = 13
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].Version = 3
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		AttemptID:        "attempt-1",
		Sequence:         1,
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "evidence delivered",
		Evidence:         []string{"artifact://evidence"},
	}}
	snapshot.WorkItems = append(snapshot.WorkItems, protocol.WorkItem{
		ID:          "work-2",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "parallel",
		Kind:        protocol.WorkItemKindProduce,
	})
	snapshot.WorkItemStates = append(snapshot.WorkItemStates, protocol.WorkItemState{
		WorkItemID:    "work-2",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-2",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	})
	snapshot.WorkItemSpecs = append(snapshot.WorkItemSpecs, protocol.WorkItemSpec{
		ID:                 "spec-2",
		WorkItemID:         "work-2",
		ExecutionID:        snapshot.Execution.ID,
		Version:            1,
		Subject:            "Parallel work",
		Objective:          "Complete independent work",
		Deliverable:        "Parallel artifact",
		AcceptanceCriteria: []string{"artifact verified"},
	})
	snapshot.PlanItems = append(snapshot.PlanItems, protocol.ExecutionPlanItem{
		PlanID:      "plan-1",
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-2",
		SpecID:      "spec-2",
		Required:    true,
	})
	snapshot.Assignments = append(snapshot.Assignments, protocol.WorkAssignment{
		ID:           "assignment-2",
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       "plan-1",
		WorkItemID:   "work-2",
		SpecID:       "spec-2",
		OwnerAgentID: "agent-parallel",
		Strategy:     protocol.AssignmentStrategySelf,
		Status:       protocol.WorkAssignmentStatusActive,
		Version:      1,
	})
	reviewCalled := false
	repository := &fakeRepository{
		snapshot: snapshot,
		block: func(
			_ context.Context,
			command orchestrationstore.BlockCommand,
		) (*protocol.ExecutionSnapshot, error) {
			if command.State.WorkItemID != "work-2" ||
				command.State.CurrentSpecID != "spec-2" {
				t.Fatalf("Block reached repository for review-locked work = %#v", command)
			}
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			result.WorkItemStates[1] = command.State
			result.Assignments[1].Status = protocol.WorkAssignmentStatusReleased
			return result, nil
		},
		takeover: func(
			_ context.Context,
			command orchestrationstore.TakeoverCommand,
		) (*protocol.ExecutionSnapshot, error) {
			if command.CurrentAssignmentID != "assignment-2" ||
				command.Replacement.WorkItemID != "work-2" {
				t.Fatalf("Takeover reached repository for review-locked work = %#v", command)
			}
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			result.Assignments[1].Status = protocol.WorkAssignmentStatusReleased
			result.Assignments = append(result.Assignments, command.Replacement)
			return result, nil
		},
		review: func(
			_ context.Context,
			command orchestrationstore.ReviewCommand,
		) (*protocol.ExecutionSnapshot, error) {
			reviewCalled = true
			result := cloneExecutionSnapshot(snapshot)
			result.Execution.Version++
			result.Assignments[0].Status = protocol.WorkAssignmentStatusCompleted
			result.Acceptances = []protocol.WorkAcceptance{command.Acceptance}
			result.CompletionBlockers = []string{"another required Work Item remains"}
			return result, nil
		},
	}
	service := testService(repository)
	actor := coordinatorActor()

	blocked, err := service.BlockWork(context.Background(), actor, BlockWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-block-pending-review",
		LogicalKey:       "research",
		Reason:           "wait for another source",
		NeededInput:      "source-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Outcome != MutationRejected ||
		blocked.ReasonCode != ErrorCodeCompletionBlocked ||
		!strings.Contains(blocked.Message, "review the pending Submission") {
		t.Fatalf("BlockWork with pending Submission = %#v", blocked)
	}

	takenOver, err := service.TakeOverWork(context.Background(), actor, TakeOverWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-takeover-pending-review",
		LogicalKey:       "research",
		TargetAgentID:    "agent-replacement",
		Reason:           "replace owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	if takenOver.Outcome != MutationRejected ||
		takenOver.ReasonCode != ErrorCodeCompletionBlocked ||
		!strings.Contains(takenOver.Message, "review the pending Submission") {
		t.Fatalf("TakeOverWork with pending Submission = %#v", takenOver)
	}

	safeBlocked, err := service.BlockWork(context.Background(), actor, BlockWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-block-safe-parallel",
		LogicalKey:       "parallel",
		Reason:           "parallel approval missing",
		NeededInput:      "parallel-approval",
	})
	if err != nil {
		t.Fatal(err)
	}
	if safeBlocked.Outcome != MutationApplied {
		t.Fatalf("BlockWork for unrelated safe Work Item = %#v", safeBlocked)
	}

	safeTakenOver, err := service.TakeOverWork(context.Background(), actor, TakeOverWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-takeover-safe-parallel",
		LogicalKey:       "parallel",
		TargetAgentID:    actor.AgentID,
		Strategy:         protocol.AssignmentStrategySelf,
		Reason:           "finish independent work directly",
	})
	if err != nil {
		t.Fatal(err)
	}
	if safeTakenOver.Outcome != MutationApplied {
		t.Fatalf("TakeOverWork for unrelated safe Work Item = %#v", safeTakenOver)
	}

	reviewed, err := service.ReviewWork(context.Background(), actor, ReviewWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "tool-review-after-rejections",
		SubmissionID:     "submission-1",
		Decision:         protocol.WorkAcceptanceAccepted,
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "sources cited",
			Passed:    true,
			Evidence:  []string{"artifact://evidence"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Outcome != MutationApplied || !reviewCalled ||
		len(reviewed.Snapshot.Acceptances) != 1 {
		t.Fatalf("review after rejected state mutations = %#v called=%t", reviewed, reviewCalled)
	}
}

func TestServiceGoalExecutionCompletionBlockerUsesObjectiveRevision(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.GoalID = "goal-1"
	snapshot.Execution.GoalObjectiveRevision = 4
	snapshot.CompletionBlockers = []string{"work_item:work-1:required_not_accepted"}
	var gotGoal string
	var gotRevision int64
	repository := &fakeRepository{
		snapshot: snapshot,
		findCurrentByGoal: func(
			_ context.Context,
			goalID string,
			revision int64,
		) (*protocol.Execution, error) {
			gotGoal = goalID
			gotRevision = revision
			item := snapshot.Execution
			return &item, nil
		},
	}
	service := testService(repository)
	blocker, err := service.GoalExecutionCompletionBlocker(context.Background(), protocol.Goal{
		ID:         "goal-1",
		SessionKey: "session-1",
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(4),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotGoal != "goal-1" || gotRevision != 4 {
		t.Fatalf("lookup = %q/%d", gotGoal, gotRevision)
	}
	if blocker != "execution_work_graph:execution-1:work_item:work-1:required_not_accepted" {
		t.Fatalf("blocker = %q", blocker)
	}
}

func TestRoomCoordinatorSelfAssignmentReturnsExplicitWorkBindingReceipt(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.ScopeKind = protocol.ExecutionScopeRoom
	snapshot.Execution.RoomID = "room-1"
	snapshot.Execution.ConversationID = "conversation-1"
	snapshot.Assignments = nil
	snapshot.Attempts = nil
	snapshot.ReadyWorkItemIDs = []string{"work-1"}
	repository := &fakeRepository{snapshot: snapshot}
	repository.assign = func(
		_ context.Context,
		command orchestrationstore.AssignCommand,
	) (*protocol.ExecutionSnapshot, error) {
		result := cloneExecutionSnapshot(snapshot)
		result.Execution.Version++
		result.Assignments = append(result.Assignments, command.Assignment)
		result.Attempts = append(result.Attempts, *command.RootAttempt)
		return result, nil
	}
	service := testService(repository)
	actor := ActorContext{
		OwnerUserID: snapshot.Execution.OwnerUserID,
		SessionKey:  snapshot.Execution.SessionKey,
		AgentID:     snapshot.Execution.CoordinatorAgentID,
		Role:        ExecutionActorCoordinator, ActorKind: protocol.ExecutionActorAgent,
		ScopeKind: protocol.ExecutionScopeRoom,
		RoomID:    snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID,
		RootRoundID: "root-1", RuntimeRoundID: "runtime-1", AgentRoundID: "agent-round-1",
	}
	if err := service.mintRuntimeCoordination(actor, snapshot.Execution.ID); err != nil {
		t.Fatal(err)
	}
	result, err := service.AssignWork(context.Background(), actor, AssignWorkInput{
		ExecutionID: snapshot.Execution.ID, SnapshotRevision: snapshot.Execution.Version,
		CommandID: "assign-self-room", WorkItemID: "work-1",
		TargetAgentID: actor.AgentID, Strategy: protocol.AssignmentStrategySelf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied || result.WorkBinding == nil ||
		result.WorkBinding.Binding == nil || result.WorkBinding.Clear {
		t.Fatalf("self assignment result = %#v", result)
	}
	binding := result.WorkBinding.Binding
	if binding.ExecutionID != snapshot.Execution.ID || binding.WorkItemID != "work-1" ||
		binding.AssignmentID == "" || binding.AttemptID == "" || binding.DispatchID != "" {
		t.Fatalf("self WorkBinding receipt = %#v", binding)
	}
	repository.snapshot = result.Snapshot
	replay, err := service.AssignWork(context.Background(), actor, AssignWorkInput{
		ExecutionID: result.Snapshot.Execution.ID, SnapshotRevision: result.Snapshot.Execution.Version,
		CommandID: "assign-self-room-recover-receipt", WorkItemID: "work-1",
		TargetAgentID: actor.AgentID, Strategy: protocol.AssignmentStrategySelf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replay.Outcome != MutationNoOp || replay.WorkBinding == nil ||
		replay.WorkBinding.Binding == nil ||
		replay.WorkBinding.Binding.AssignmentID != binding.AssignmentID ||
		replay.WorkBinding.Binding.AttemptID != binding.AttemptID {
		t.Fatalf("self Assignment receipt recovery = %#v", replay)
	}

	dmSnapshot := cloneExecutionSnapshot(snapshot)
	dmSnapshot.Execution.ScopeKind = protocol.ExecutionScopeDM
	dmResult := AppliedResult(dmSnapshot, nil, nil)
	dmResult = withRoomSelfWorkBindingReceipt(
		coordinatorActor(),
		dmResult,
		"work-1",
	)
	if dmResult.WorkBinding != nil {
		t.Fatalf("DM received Room WorkBinding receipt: %#v", dmResult.WorkBinding)
	}
}

func testService(repository Repository) *Service {
	service := NewService(repository)
	counts := make(map[string]int)
	service.newID = func(kind string) string {
		counts[kind]++
		return kind + "-" + string(rune('0'+counts[kind]))
	}
	return service
}

func coordinatorActor() ActorContext {
	return ActorContext{
		OwnerUserID: "owner-1",
		SessionKey:  "session-1",
		AgentID:     "agent-lead",
		Role:        ExecutionActorCoordinator,
		ScopeKind:   protocol.ExecutionScopeDM,
	}
}

func executionSnapshot() *protocol.ExecutionSnapshot {
	return &protocol.ExecutionSnapshot{
		Execution: protocol.Execution{
			ID:                 "execution-1",
			OwnerUserID:        "owner-1",
			SessionKey:         "session-1",
			ScopeKind:          protocol.ExecutionScopeDM,
			CoordinatorAgentID: "agent-lead",
			Origin:             protocol.ExecutionOriginUserRequest,
			Objective:          "ship orchestration",
			Status:             protocol.ExecutionStatusActive,
			Version:            1,
		},
	}
}

func assignedExecutionSnapshot() *protocol.ExecutionSnapshot {
	snapshot := executionSnapshot()
	snapshot.Execution.Version = 10
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-1",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
		Version:     1,
	}
	snapshot.WorkItems = []protocol.WorkItem{{
		ID:          "work-1",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "research",
		Kind:        protocol.WorkItemKindProduce,
	}}
	snapshot.WorkItemStates = []protocol.WorkItemState{{
		WorkItemID:    "work-1",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-1",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	}}
	snapshot.WorkItemSpecs = []protocol.WorkItemSpec{{
		ID:                 "spec-1",
		WorkItemID:         "work-1",
		ExecutionID:        snapshot.Execution.ID,
		Version:            1,
		Subject:            "Research",
		Objective:          "Collect evidence",
		Deliverable:        "Evidence set",
		AcceptanceCriteria: []string{"sources cited"},
		SpecHash:           "hash-1",
	}}
	snapshot.PlanItems = []protocol.ExecutionPlanItem{{
		PlanID:      "plan-1",
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-1",
		SpecID:      "spec-1",
		Required:    true,
	}}
	snapshot.Assignments = []protocol.WorkAssignment{{
		ID:           "assignment-1",
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		OwnerAgentID: "agent-worker",
		Strategy:     protocol.AssignmentStrategyRoomMember,
		Status:       protocol.WorkAssignmentStatusAssigned,
		Version:      1,
	}}
	snapshot.Attempts = []protocol.WorkAttempt{{
		ID:              "attempt-1",
		ExecutionID:     snapshot.Execution.ID,
		PlanID:          "plan-1",
		WorkItemID:      "work-1",
		SpecID:          "spec-1",
		AssignmentID:    "assignment-1",
		ExecutorKind:    protocol.AttemptExecutorAgent,
		ExecutorAgentID: "agent-worker",
		Status:          protocol.WorkAttemptStatusPending,
		Version:         1,
	}}
	return snapshot
}

func cloneExecutionSnapshot(source *protocol.ExecutionSnapshot) *protocol.ExecutionSnapshot {
	result := *source
	result.WorkItems = append([]protocol.WorkItem(nil), source.WorkItems...)
	result.WorkItemStates = append([]protocol.WorkItemState(nil), source.WorkItemStates...)
	result.WorkItemSpecs = append([]protocol.WorkItemSpec(nil), source.WorkItemSpecs...)
	result.PlanItems = append([]protocol.ExecutionPlanItem(nil), source.PlanItems...)
	result.Dependencies = append([]protocol.ExecutionPlanDependency(nil), source.Dependencies...)
	result.Assignments = append([]protocol.WorkAssignment(nil), source.Assignments...)
	result.Attempts = append([]protocol.WorkAttempt(nil), source.Attempts...)
	result.Submissions = append([]protocol.WorkSubmission(nil), source.Submissions...)
	result.ReviewDispatches = append(
		[]protocol.ExecutionReviewDispatch(nil),
		source.ReviewDispatches...,
	)
	result.CancellationDispatches = append(
		[]protocol.ExecutionCancellationDispatch(nil),
		source.CancellationDispatches...,
	)
	result.Acceptances = append([]protocol.WorkAcceptance(nil), source.Acceptances...)
	return &result
}

type fakeRepository struct {
	snapshot          *protocol.ExecutionSnapshot
	create            func(context.Context, orchestrationstore.CreateCommand) (*protocol.ExecutionSnapshot, error)
	createWithPlan    func(context.Context, orchestrationstore.CreateWithPlanCommand) (*protocol.ExecutionSnapshot, error)
	replaceWithPlan   func(context.Context, orchestrationstore.ReplaceWithPlanCommand) (*protocol.ExecutionSnapshot, error)
	abandon           func(context.Context, orchestrationstore.AbandonCommand) (*protocol.ExecutionSnapshot, error)
	supersedeGoal     func(context.Context, orchestrationstore.SupersedeGoalRevisionCommand) (*protocol.ExecutionSnapshot, error)
	fenceGoalIdentity func(context.Context, orchestrationstore.FenceGoalExecutionIdentityCommand) (bool, error)
	findCurrentByGoal func(context.Context, string, int64) (*protocol.Execution, error)
	writePlan         func(context.Context, orchestrationstore.WritePlanCommand) (*protocol.ExecutionSnapshot, error)
	assign            func(context.Context, orchestrationstore.AssignCommand) (*protocol.ExecutionSnapshot, error)
	startAttempt      func(context.Context, orchestrationstore.StartAttemptCommand) (*protocol.ExecutionSnapshot, error)
	finishAttempt     func(context.Context, orchestrationstore.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error)
	scheduleSubagent  func(context.Context, orchestrationstore.ScheduleSubagentReconciliationCommand) (*protocol.ExecutionSnapshot, error)
	listExpired       func(context.Context, time.Time, int) ([]protocol.WorkAttempt, error)
	listOrphaned      func(context.Context, time.Time, int) ([]protocol.WorkAttempt, error)
	submit            func(context.Context, orchestrationstore.SubmitCommand) (*protocol.ExecutionSnapshot, error)
	review            func(context.Context, orchestrationstore.ReviewCommand) (*protocol.ExecutionSnapshot, error)
	block             func(context.Context, orchestrationstore.BlockCommand) (*protocol.ExecutionSnapshot, error)
	resume            func(context.Context, orchestrationstore.ResumeCommand) (*protocol.ExecutionSnapshot, error)
	takeover          func(context.Context, orchestrationstore.TakeoverCommand) (*protocol.ExecutionSnapshot, error)
	bindGoal          func(context.Context, orchestrationstore.BindGoalCommand) (*protocol.ExecutionSnapshot, error)
	complete          func(context.Context, orchestrationstore.CompleteCommand) (*protocol.ExecutionSnapshot, error)
}

func (f *fakeRepository) Create(
	ctx context.Context,
	command orchestrationstore.CreateCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.create == nil {
		return nil, errors.New("unexpected Create")
	}
	return f.create(ctx, command)
}

func (f *fakeRepository) CreateWithPlan(
	ctx context.Context,
	command orchestrationstore.CreateWithPlanCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.createWithPlan == nil {
		return nil, errors.New("unexpected CreateWithPlan")
	}
	return f.createWithPlan(ctx, command)
}

func (f *fakeRepository) ReplaceWithPlan(
	ctx context.Context,
	command orchestrationstore.ReplaceWithPlanCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.replaceWithPlan == nil {
		return nil, errors.New("unexpected ReplaceWithPlan")
	}
	return f.replaceWithPlan(ctx, command)
}

func (f *fakeRepository) Abandon(
	ctx context.Context,
	command orchestrationstore.AbandonCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.abandon == nil {
		return nil, errors.New("unexpected Abandon")
	}
	return f.abandon(ctx, command)
}

func (f *fakeRepository) SupersedeGoalRevision(
	ctx context.Context,
	command orchestrationstore.SupersedeGoalRevisionCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.supersedeGoal == nil {
		return nil, errors.New("unexpected SupersedeGoalRevision")
	}
	return f.supersedeGoal(ctx, command)
}

func (f *fakeRepository) FenceGoalExecutionIdentity(
	ctx context.Context,
	command orchestrationstore.FenceGoalExecutionIdentityCommand,
) (bool, error) {
	if f.fenceGoalIdentity == nil {
		return false, errors.New("unexpected FenceGoalExecutionIdentity")
	}
	return f.fenceGoalIdentity(ctx, command)
}

func (f *fakeRepository) Get(context.Context, string) (*protocol.Execution, error) {
	if f.snapshot == nil {
		return nil, nil
	}
	item := f.snapshot.Execution
	return &item, nil
}

func (f *fakeRepository) FindCurrent(
	context.Context,
	string,
	string,
) (*protocol.Execution, error) {
	if f.snapshot == nil {
		return nil, nil
	}
	item := f.snapshot.Execution
	return &item, nil
}

func (f *fakeRepository) FindCurrentByGoal(
	ctx context.Context,
	goalID string,
	revision int64,
) (*protocol.Execution, error) {
	if f.findCurrentByGoal == nil {
		return nil, nil
	}
	return f.findCurrentByGoal(ctx, goalID, revision)
}

func (f *fakeRepository) GetSnapshot(
	context.Context,
	string,
) (*protocol.ExecutionSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeRepository) WritePlan(
	ctx context.Context,
	command orchestrationstore.WritePlanCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.writePlan == nil {
		return nil, errors.New("unexpected WritePlan")
	}
	return f.writePlan(ctx, command)
}

func (f *fakeRepository) Assign(
	ctx context.Context,
	command orchestrationstore.AssignCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.assign == nil {
		return nil, errors.New("unexpected Assign")
	}
	return f.assign(ctx, command)
}

func (f *fakeRepository) StartAttempt(
	ctx context.Context,
	command orchestrationstore.StartAttemptCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.startAttempt == nil {
		return nil, errors.New("unexpected StartAttempt")
	}
	return f.startAttempt(ctx, command)
}

func (f *fakeRepository) FinishAttempt(
	ctx context.Context,
	command orchestrationstore.FinishAttemptCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.finishAttempt == nil {
		return nil, errors.New("unexpected FinishAttempt")
	}
	return f.finishAttempt(ctx, command)
}

func (f *fakeRepository) ScheduleSubagentReconciliation(
	ctx context.Context,
	command orchestrationstore.ScheduleSubagentReconciliationCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.scheduleSubagent == nil {
		return nil, errors.New("unexpected ScheduleSubagentReconciliation")
	}
	return f.scheduleSubagent(ctx, command)
}

func (f *fakeRepository) ListExpiredSubagentAttempts(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]protocol.WorkAttempt, error) {
	if f.listExpired == nil {
		return nil, errors.New("unexpected ListExpiredSubagentAttempts")
	}
	return f.listExpired(ctx, now, limit)
}

func (f *fakeRepository) ListOrphanedSubagentAttempts(
	ctx context.Context,
	createdBefore time.Time,
	limit int,
) ([]protocol.WorkAttempt, error) {
	if f.listOrphaned == nil {
		return nil, errors.New("unexpected ListOrphanedSubagentAttempts")
	}
	return f.listOrphaned(ctx, createdBefore, limit)
}

func (f *fakeRepository) Submit(
	ctx context.Context,
	command orchestrationstore.SubmitCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.submit == nil {
		return nil, errors.New("unexpected Submit")
	}
	return f.submit(ctx, command)
}

func (f *fakeRepository) Review(
	ctx context.Context,
	command orchestrationstore.ReviewCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.review == nil {
		return nil, errors.New("unexpected Review")
	}
	return f.review(ctx, command)
}

func (f *fakeRepository) Block(
	ctx context.Context,
	command orchestrationstore.BlockCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.block == nil {
		return nil, errors.New("unexpected Block")
	}
	return f.block(ctx, command)
}

func (f *fakeRepository) Resume(
	ctx context.Context,
	command orchestrationstore.ResumeCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.resume == nil {
		return nil, errors.New("unexpected Resume")
	}
	return f.resume(ctx, command)
}

func (f *fakeRepository) Takeover(
	ctx context.Context,
	command orchestrationstore.TakeoverCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.takeover == nil {
		return nil, errors.New("unexpected Takeover")
	}
	return f.takeover(ctx, command)
}

func (f *fakeRepository) BindGoal(
	ctx context.Context,
	command orchestrationstore.BindGoalCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.bindGoal == nil {
		return nil, errors.New("unexpected BindGoal")
	}
	return f.bindGoal(ctx, command)
}

func (f *fakeRepository) Complete(
	ctx context.Context,
	command orchestrationstore.CompleteCommand,
) (*protocol.ExecutionSnapshot, error) {
	if f.complete == nil {
		return nil, errors.New("unexpected Complete")
	}
	return f.complete(ctx, command)
}

type goalPromotionGatewayFunc func(
	context.Context,
	GoalPromotionRequest,
) (GoalPromotionBinding, error)

func (f goalPromotionGatewayFunc) PromoteExecution(
	ctx context.Context,
	request GoalPromotionRequest,
) (GoalPromotionBinding, error) {
	return f(ctx, request)
}

type goalPromotionGatewayWithAvailability struct {
	GoalPromotionGateway
	availability GoalPromotionAvailability
	err          error
}

func (g goalPromotionGatewayWithAvailability) ReadGoalPromotionAvailability(
	context.Context,
	GoalPromotionAvailabilityRequest,
) (GoalPromotionAvailability, error) {
	return g.availability, g.err
}
