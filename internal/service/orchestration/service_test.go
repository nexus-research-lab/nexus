// service_test.go 覆盖编排主服务、显式 Goal、重定目标、人工交互与证据测试。
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

// projectExecutionGraphView 是只给无 Repository 单测使用的窄入口。
func projectExecutionGraphView(
	items []protocol.ExecutionWorkItemView,
) protocol.ExecutionGraphView {
	return projectExecutionGraphViewWithHistory(items, protocol.ExecutionWorkGraphHistory{})
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

func TestEnsureCreatesExecutionOnActiveExplicitGoalChain(t *testing.T) {
	var created orchestrationstore.CreateCommand
	repository := &fakeRepository{
		create: func(
			_ context.Context,
			command orchestrationstore.CreateCommand,
		) (*protocol.ExecutionSnapshot, error) {
			created = command
			item := command.Execution
			item.Version = 1
			return &protocol.ExecutionSnapshot{Execution: item}, nil
		},
	}
	service := testService(repository)
	service.SetExplicitGoalBindingGateway(explicitGoalGatewayFunc(func(
		_ context.Context,
		request ExplicitGoalBindingRequest,
	) (*ExplicitGoalBinding, error) {
		if request.ExistingExecution ||
			request.CandidateExecutionID != "execution-1" ||
			request.Objective != "Ship orchestration" {
			t.Fatalf("binding request = %#v", request)
		}
		return &ExplicitGoalBinding{
			ExecutionID:           "execution-reserved",
			GoalID:                "goal-explicit",
			GoalObjectiveRevision: 3,
			ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
			ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		}, nil
	}))
	result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
		CommandID:          "plan-command",
		Objective:          "Ship orchestration",
		CompletionCriteria: []string{"accepted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		created.Execution.ID != "execution-reserved" ||
		created.Execution.GoalID != "goal-explicit" ||
		created.Execution.GoalObjectiveRevision != 3 ||
		created.Execution.GoalActivationOrigin != protocol.GoalActivationOriginUserExplicit ||
		created.Execution.GoalActivationReason != protocol.GoalActivationReasonPersistenceRequested {
		t.Fatalf("result=%#v created=%#v", result, created.Execution)
	}
}

func TestEnsureBindsCurrentTransientExecutionToExplicitGoal(t *testing.T) {
	snapshot := executionSnapshot()
	var bound orchestrationstore.BindGoalCommand
	repository := &fakeRepository{
		snapshot: snapshot,
		bindGoal: func(
			_ context.Context,
			command orchestrationstore.BindGoalCommand,
		) (*protocol.ExecutionSnapshot, error) {
			bound = command
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
	gateway := &confirmingGoalBindingGateway{
		binding: ExplicitGoalBinding{
			ExecutionID:           snapshot.Execution.ID,
			GoalID:                "goal-explicit",
			GoalObjectiveRevision: 1,
			ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
			ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		},
	}
	service.SetExplicitGoalBindingGateway(gateway)
	result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
		CommandID: "create-goal-command",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		bound.ExpectedExecutionVersion != snapshot.Execution.Version ||
		bound.Meta.CommandID != "create-goal-command:bind-explicit-goal" ||
		bound.Execution.GoalID != "goal-explicit" {
		t.Fatalf("result=%#v command=%#v", result, bound)
	}
	if gateway.prepareCalls != 1 || gateway.confirmCalls != 1 ||
		gateway.lastConfirmation.ExecutionID != snapshot.Execution.ID {
		t.Fatalf("binding gateway = %#v", gateway)
	}
}

func TestEnsureRejectsIncompatibleExplicitGoalWithoutCreatingExecution(t *testing.T) {
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
	service.SetExplicitGoalBindingGateway(explicitGoalGatewayFunc(func(
		context.Context,
		ExplicitGoalBindingRequest,
	) (*ExplicitGoalBinding, error) {
		return nil, fmtExplicitGoalObjectiveConflict()
	}))
	result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
		CommandID:          "plan-command",
		Objective:          "Different objective",
		CompletionCriteria: []string{"accepted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeGoalObjectiveConflict ||
		created {
		t.Fatalf("result=%#v created=%t", result, created)
	}
}

func TestGoalExecutionCompletionBlockerSeparatesReservationFromConfirmedBinding(t *testing.T) {
	service := testService(&fakeRepository{})
	blocker, err := service.GoalExecutionCompletionBlocker(context.Background(), protocol.Goal{
		ID:         "goal-explicit",
		SessionKey: "session-1",
		Metadata: map[string]any{
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocker != "" {
		t.Fatalf("unbound explicit Goal blocker = %q, want empty", blocker)
	}
	legacyGoal := protocol.Goal{
		ID:         "goal-explicit-legacy",
		SessionKey: "session-legacy",
		Metadata: map[string]any{
			protocol.GoalMetadataExplicitCommand:  "explicit_goal_legacy",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	}
	blocker, err = service.GoalExecutionCompletionBlocker(context.Background(), legacyGoal)
	if err != nil {
		t.Fatal(err)
	}
	if blocker != "" {
		t.Fatalf("legacy reserved Goal blocker = %q, want empty", blocker)
	}

	snapshot := executionSnapshot()
	snapshot.Execution.GoalID = "goal-explicit"
	snapshot.Execution.GoalObjectiveRevision = 1
	snapshot.CompletionBlockers = []string{"work_item:W1:required_not_accepted"}
	service = testService(&fakeRepository{snapshot: snapshot})
	goal := protocol.Goal{
		ID:         "goal-explicit",
		SessionKey: "session-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: snapshot.Execution.ID,
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
		},
	}
	blocker, err = service.GoalExecutionCompletionBlocker(context.Background(), goal)
	if err != nil {
		t.Fatal(err)
	}
	if blocker != "execution_work_graph:execution-1:work_item:W1:required_not_accepted" {
		t.Fatalf("bound blocker = %q", blocker)
	}
	snapshot.Execution.Status = protocol.ExecutionStatusCompleted
	snapshot.CompletionBlockers = nil
	blocker, err = service.GoalExecutionCompletionBlocker(context.Background(), goal)
	if err != nil || blocker != "" {
		t.Fatalf("completed blocker = %q, err=%v", blocker, err)
	}
}

type explicitGoalGatewayFunc func(
	context.Context,
	ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error)

func (f explicitGoalGatewayFunc) PrepareExplicitGoalBinding(
	ctx context.Context,
	request ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error) {
	return f(ctx, request)
}

type confirmingGoalBindingGateway struct {
	binding          ExplicitGoalBinding
	prepareCalls     int
	confirmCalls     int
	confirmErrOnce   error
	lastConfirmation GoalExecutionBindingConfirmation
}

func (g *confirmingGoalBindingGateway) PrepareExplicitGoalBinding(
	_ context.Context,
	_ ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error) {
	g.prepareCalls++
	result := g.binding
	return &result, nil
}

func (g *confirmingGoalBindingGateway) ConfirmGoalExecutionBinding(
	_ context.Context,
	confirmation GoalExecutionBindingConfirmation,
) error {
	g.confirmCalls++
	g.lastConfirmation = confirmation
	if g.confirmErrOnce != nil {
		err := g.confirmErrOnce
		g.confirmErrOnce = nil
		return err
	}
	return nil
}

func fmtExplicitGoalObjectiveConflict() error {
	return errors.Join(
		ErrExplicitGoalObjectiveConflict,
		errors.New("active Goal and Execution objectives differ"),
	)
}

func TestServiceGoalRevisionSupersedeFailsClosedIfMaterializationClaimHasNoRow(t *testing.T) {
	service := testService(&fakeRepository{
		fenceGoalIdentity: func(
			context.Context,
			orchestrationstore.FenceGoalExecutionIdentityCommand,
		) (bool, error) {
			return false, nil
		},
	})
	_, err := service.SupersedeGoalRevision(context.Background(), GoalRevisionSupersedeInput{
		ExecutionID:              "execution-missing",
		GoalID:                   "goal-1",
		OldGoalObjectiveRevision: 1,
		NewGoalObjectiveRevision: 2,
		SuccessorExecutionID:     "execution-successor",
		CommandID:                "goal-retarget",
		Reason:                   "user changed the Goal objective",
	})
	var domainErr *DomainError
	if !errors.As(err, &domainErr) ||
		domainErr.Code != ErrorCodeGoalBindingConflict {
		t.Fatalf("dangling materialization error = %v", err)
	}
}

func TestMatchAwaitingHumanAttemptsBindsOnlyRunningRootAttemptOfSameAgent(t *testing.T) {
	createdAt := time.Date(2026, 11, 27, 3, 49, 6, 0, time.UTC)
	attempts := []protocol.WorkAttempt{
		{ID: "running", Status: protocol.WorkAttemptStatusRunning, ExecutorAgentID: "owner", RuntimeSessionKey: "rt-owner"},
		{ID: "finished", Status: protocol.WorkAttemptStatusInterrupted, ExecutorAgentID: "owner", RuntimeSessionKey: "rt-owner"},
		{ID: "child", Status: protocol.WorkAttemptStatusRunning, ExecutorAgentID: "owner", ParentAttemptID: "running"},
		{ID: "other", Status: protocol.WorkAttemptStatusRunning, ExecutorAgentID: "other", RuntimeSessionKey: "rt-other"},
	}
	interactions := []PendingHumanInteraction{
		{RequestID: "perm_owner", AgentID: "owner", RuntimeSessionKey: "rt-owner", ToolName: "AskUserQuestion", CreatedAt: createdAt},
		{RequestID: "perm_foreign_session", AgentID: "owner", RuntimeSessionKey: "rt-elsewhere", ToolName: "Bash", CreatedAt: createdAt},
	}

	matched := matchAwaitingHumanAttempts(attempts, interactions)

	if len(matched) != 1 {
		t.Fatalf("matched attempts = %v, want only the running root attempt", matched)
	}
	got := matched["running"]
	if len(got) != 1 || got[0].RequestID != "perm_owner" {
		t.Fatalf("running attempt interactions = %+v, want perm_owner only", got)
	}
	if awaitingHumanKind(got) != HumanInteractionQuestion {
		t.Fatalf("kind = %q, want question", awaitingHumanKind(got))
	}
}

func TestRenderAwaitingHumanWarnsCoordinatorAgainstTakeover(t *testing.T) {
	var output strings.Builder
	renderAwaitingHuman(&output, []PendingHumanInteraction{{
		RequestID:    "perm_1",
		ToolName:     "AskUserQuestion",
		AgentRoundID: "agent_round_b837",
		CreatedAt:    time.Date(2026, 11, 27, 3, 49, 6, 0, time.UTC),
	}}, 6)

	rendered := output.String()
	for _, want := range []string{
		`<awaiting_human count="1">`,
		`<interaction kind="question" tool="AskUserQuestion" request_id="perm_1" agent_round_id="agent_round_b837" since="2026-11-27T03:49:06Z" />`,
		"Do not take_over_work",
		"</awaiting_human>",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered awaiting_human lacks %q:\n%s", want, rendered)
		}
	}
}

type evidenceTestRepository struct {
	*fakeRepository
	recordCalls []orchestrationstore.RecordEvidenceCommand
	currentID   string
	snapshots   map[string]*protocol.ExecutionSnapshot
}

func (r *evidenceTestRepository) FindCurrent(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.Execution, error) {
	if r.currentID == "" {
		return r.fakeRepository.FindCurrent(ctx, ownerUserID, sessionKey)
	}
	snapshot := r.snapshots[r.currentID]
	if snapshot == nil {
		return nil, nil
	}
	item := snapshot.Execution
	return &item, nil
}

func (r *evidenceTestRepository) GetSnapshot(
	ctx context.Context,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	if r.snapshots == nil {
		return r.fakeRepository.GetSnapshot(ctx, executionID)
	}
	return r.snapshots[executionID], nil
}

func (r *evidenceTestRepository) RecordEvidence(
	_ context.Context,
	command orchestrationstore.RecordEvidenceCommand,
) (*protocol.ExecutionSnapshot, error) {
	r.recordCalls = append(r.recordCalls, command)
	snapshot := r.snapshot
	if r.snapshots != nil {
		snapshot = r.snapshots[command.ExecutionID]
	}
	snapshot.Execution.Version++
	if snapshot.Execution.Metadata == nil {
		snapshot.Execution.Metadata = map[string]any{}
	}
	snapshot.Execution.Metadata[command.MetadataKey] = true
	return snapshot, nil
}

func TestServiceRecordPersistenceEvidenceUsesRuntimeActorAndSuppressesDuplicates(t *testing.T) {
	snapshot := executionSnapshot()
	repository := &evidenceTestRepository{
		fakeRepository: &fakeRepository{snapshot: snapshot},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.RootRoundID = "round-1"
	actor.RuntimeRoundID = "round-1"
	actor.AgentRoundID = "agent-round-1"
	snapshot.Execution.RootRoundID = actor.RootRoundID

	if err := service.RecordPersistenceEvidence(
		context.Background(),
		actor,
		PersistenceEvidenceContextBoundary,
		"runtime:agent-round-1:compact-boundary",
	); err != nil {
		t.Fatal(err)
	}
	if len(repository.recordCalls) != 1 {
		t.Fatalf("record calls = %#v", repository.recordCalls)
	}
	command := repository.recordCalls[0]
	if command.MetadataKey != ExecutionMetadataContextBoundaryEvidence ||
		command.Meta.ActorKind != protocol.ExecutionActorRuntime ||
		command.Meta.CommandID != "runtime:agent-round-1:compact-boundary:record-context_boundary" {
		t.Fatalf("record command = %#v", command)
	}
	if err := service.RecordPersistenceEvidence(
		context.Background(),
		actor,
		PersistenceEvidenceContextBoundary,
		"runtime:agent-round-1:compact-boundary",
	); err != nil {
		t.Fatal(err)
	}
	if len(repository.recordCalls) != 1 {
		t.Fatalf("duplicate boundary wrote again: %#v", repository.recordCalls)
	}
}

func TestServiceRecordPersistenceEvidenceNeverRetargetsPredecessorToSuccessor(t *testing.T) {
	predecessor := executionSnapshot()
	predecessor.Execution.ID = "execution-old"
	predecessor.Execution.RootRoundID = "old-root"
	predecessor.Execution.Status = protocol.ExecutionStatusSuperseded
	successor := executionSnapshot()
	successor.Execution.ID = "execution-new"
	successor.Execution.RootRoundID = "new-root"
	repository := &evidenceTestRepository{
		fakeRepository: &fakeRepository{snapshot: successor},
		currentID:      successor.Execution.ID,
		snapshots: map[string]*protocol.ExecutionSnapshot{
			predecessor.Execution.ID: predecessor,
			successor.Execution.ID:   successor,
		},
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.ExecutionID = predecessor.Execution.ID
	actor.RootRoundID = predecessor.Execution.RootRoundID
	actor.RuntimeRoundID = "old-runtime-round"
	actor.AgentRoundID = "old-agent-round"

	if err := service.RecordPersistenceEvidence(
		context.Background(),
		actor,
		PersistenceEvidenceContextBoundary,
		"runtime:old-agent-round:compact-boundary",
	); err != nil {
		t.Fatal(err)
	}
	if len(repository.recordCalls) != 0 {
		t.Fatalf("late predecessor evidence reached successor: %#v", repository.recordCalls)
	}
	if metadataBool(
		successor.Execution.Metadata,
		ExecutionMetadataContextBoundaryEvidence,
	) {
		t.Fatal("successor was marked by predecessor compact boundary")
	}
}
