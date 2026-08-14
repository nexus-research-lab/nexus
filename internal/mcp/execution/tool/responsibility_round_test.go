package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	goalmcp "github.com/nexus-research-lab/nexus/internal/mcp/goal"
	goalcontract "github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestSameRoundReviewRetargetSuccessorPlanGetAssign(t *testing.T) {
	predecessor := executionSnapshot(8)
	predecessor.Execution.ID = "execution-old"
	predecessor.Execution.GoalID = "goal-1"
	predecessor.Execution.GoalObjectiveRevision = 1
	predecessor.Execution.Status = protocol.ExecutionStatusActive
	predecessor.Execution.Objective = "Old objective"
	successor := executionSnapshot(1)
	successor.Execution.ID = "execution-successor"
	successor.Execution.GoalID = "goal-1"
	successor.Execution.GoalObjectiveRevision = 2
	successor.Execution.Objective = "New objective"
	successor.Execution.Status = protocol.ExecutionStatusActive

	reviewBinding := &protocol.ExecutionReviewBinding{
		ExecutionID: predecessor.Execution.ID, PlanID: "plan-old", WorkItemID: "work-old",
		SpecID: "spec-old", AssignmentID: "assignment-old", SubmissionID: "submission-old",
		ReviewDispatchID: "review-dispatch-old", TargetAgentID: "agent-1",
	}
	goalState := runtimectx.NewGoalAuthorityState("goal-1", 1, predecessor.Execution.ID)
	authority := runtimectx.NewResponsibilityAuthorityState(
		goalState, predecessor.Execution.ID, nil, reviewBinding,
	)
	sctx := executionServerContext()
	sctx.ExecutionID = predecessor.Execution.ID
	sctx.ReviewBinding = reviewBinding
	sctx.GoalAuthority = goalState
	sctx.ResponsibilityAuthority = authority

	current := predecessor
	var preparedActor orchestration.ActorContext
	var materializedActor orchestration.ActorContext
	var readActor orchestration.ActorContext
	var assignedActor orchestration.ActorContext
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return current },
		review: func(orchestration.ReviewWorkInput) orchestration.MutationResult {
			result := orchestration.AppliedResult(predecessor, []string{"acceptance:accepted"}, nil)
			result.WorkBinding = &orchestration.WorkBindingReceipt{Clear: true}
			result.ResponsibilityAuthority = &orchestration.ResponsibilityAuthorityReceipt{
				ExecutionID: predecessor.Execution.ID,
			}
			return result
		},
		prepare: func(
			actor orchestration.ActorContext,
			_ orchestration.PreparePlanExecutionInput,
		) (*protocol.ExecutionPlanProposal, error) {
			preparedActor = actor
			proposal := sealedPlanProposal()
			proposal.TargetExecutionID = ""
			proposal.GoalID = "goal-1"
			proposal.GoalObjectiveRevision = 2
			return proposal, nil
		},
		materialize: func(
			actor orchestration.ActorContext,
			_ orchestration.MaterializePlanExecutionInput,
		) orchestration.MutationResult {
			materializedActor = actor
			current = successor
			result := orchestration.AppliedResult(successor, []string{"execution:execution-successor"}, nil)
			result.GoalAuthority = &orchestration.GoalAuthorityReceipt{
				GoalID: "goal-1", ObjectiveRevision: 2, ExecutionID: successor.Execution.ID,
			}
			result.ResponsibilityAuthority = &orchestration.ResponsibilityAuthorityReceipt{
				ExecutionID: successor.Execution.ID,
			}
			return result
		},
		contextActor: func(actor orchestration.ActorContext) {
			readActor = actor
		},
		assign: func(orchestration.AssignWorkInput) orchestration.MutationResult {
			assignedActor = sctx.Actor()
			return orchestration.NoOpResult(successor, "assignment already exists")
		},
	}

	if result, err := reviewWork(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{"decision": "accepted"},
		nil,
	); err != nil || result.IsError {
		t.Fatalf("review result=%#v err=%v", result, err)
	}
	afterReview := sctx.Actor()
	if afterReview.ExecutionID != predecessor.Execution.ID || afterReview.ReviewBinding != nil {
		t.Fatalf("after review actor = %#v, want predecessor coordination without review", afterReview)
	}

	goalService := &sameRoundGoalService{current: protocol.Goal{
		ID:         "goal-1",
		SessionKey: "room-session-1",
		Objective:  "Old objective",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision:     int64(1),
			protocol.GoalMetadataExecutionMode:         string(protocol.GoalExecutionModeManaged),
			protocol.GoalMetadataExecutionBindingState: string(protocol.GoalExecutionBindingStateConfirmed),
			protocol.GoalMetadataExecutionID:           predecessor.Execution.ID,
		},
	}, successorExecutionID: successor.Execution.ID}
	goalServer := goalmcp.NewServer(goalService, goalcontract.ServerContext{
		CurrentSessionKey:       "room-session-1",
		CurrentRoundID:          "round-1",
		CurrentAgentID:          "agent-1",
		GoalAuthority:           goalState,
		ResponsibilityAuthority: authority,
	})
	retargetResponse, err := goalServer.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "retarget_goal",
			"arguments": map[string]any{"objective": "New objective"},
		},
	})
	if err != nil {
		t.Fatalf("retarget transport err=%v", err)
	}
	encodedRetarget, _ := json.Marshal(retargetResponse)
	retargetResult, resultOK := retargetResponse["result"].(map[string]any)
	retargetIsError, _ := retargetResult["isError"].(bool)
	if _, rpcError := retargetResponse["error"]; rpcError || !resultOK ||
		retargetIsError || !strings.Contains(string(encodedRetarget), `"outcome":"applied"`) {
		t.Fatalf("retarget response=%s", encodedRetarget)
	}
	predecessor.Execution.Status = protocol.ExecutionStatusSuperseded
	current = predecessor
	if actor := sctx.Actor(); actor.ExecutionID != "" || actor.ReviewBinding != nil ||
		actor.GoalObjectiveRevision != 2 {
		t.Fatalf("after retarget actor = %#v, want successor planning authority", actor)
	}

	planInput := validPreparePlanToolInput()
	if result, err := preparePlanExecution(svc, sctx).ContextHandler(
		context.Background(), planInput, nil,
	); err != nil || result.IsError {
		t.Fatalf("prepare result=%#v err=%v", result, err)
	}
	if preparedActor.ExecutionID != "" || preparedActor.ReviewBinding != nil ||
		preparedActor.GoalObjectiveRevision != 2 {
		t.Fatalf("prepare actor = %#v", preparedActor)
	}
	if result, err := planExecution(svc, sctx).ContextHandler(
		context.Background(), validPlanCommitToolInput(), nil,
	); err != nil || result.IsError {
		t.Fatalf("plan result=%#v err=%v", result, err)
	}
	if materializedActor.ExecutionID != "" || materializedActor.ReviewBinding != nil ||
		materializedActor.GoalObjectiveRevision != 2 {
		t.Fatalf("materialize actor = %#v", materializedActor)
	}
	if actor := sctx.Actor(); actor.ExecutionID != successor.Execution.ID ||
		actor.ReviewBinding != nil || actor.GoalObjectiveRevision != 2 {
		t.Fatalf("successor actor = %#v", actor)
	}

	if result, err := getExecution(svc, sctx).ContextHandler(
		context.Background(), map[string]any{}, nil,
	); err != nil || result.IsError {
		t.Fatalf("get successor result=%#v err=%v", result, err)
	}
	if readActor.ExecutionID != successor.Execution.ID || readActor.ReviewBinding != nil {
		t.Fatalf("get actor = %#v", readActor)
	}
	if result, err := assignWork(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{"logical_key": "next", "target_agent_id": "agent-2"},
		nil,
	); err != nil || result.IsError {
		t.Fatalf("assign result=%#v err=%v", result, err)
	}
	if assignedActor.ExecutionID != successor.Execution.ID || assignedActor.ReviewBinding != nil {
		t.Fatalf("assign actor = %#v", assignedActor)
	}

	// Explicit historical reads remain possible, but an old superseded
	// execution is classified as terminal history rather than as a generic
	// binding mismatch. This goes through the real get_execution handler.
	current = predecessor
	oldResult, err := getExecution(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{"execution_id": predecessor.Execution.ID},
		nil,
	)
	if err != nil || oldResult.IsError {
		t.Fatalf("get predecessor result=%#v err=%v", oldResult, err)
	}
	if oldResult.StructuredContent["outcome"] != string(orchestration.MutationSuperseded) ||
		oldResult.StructuredContent["reason_code"] != string(orchestration.ErrorCodeExecutionTerminal) {
		t.Fatalf("old execution result = %#v", oldResult.StructuredContent)
	}
}

var _ contract.Service = (*fakeExecutionService)(nil)

type sameRoundGoalService struct {
	current              protocol.Goal
	successorExecutionID string
}

func (s *sameRoundGoalService) Create(
	context.Context,
	protocol.CreateGoalRequest,
) (*protocol.Goal, error) {
	return nil, nil
}

func (s *sameRoundGoalService) Current(
	context.Context,
	string,
) (*protocol.Goal, error) {
	result := s.current
	return &result, nil
}

func (s *sameRoundGoalService) CurrentOptional(
	ctx context.Context,
	sessionKey string,
) (*protocol.Goal, error) {
	return s.Current(ctx, sessionKey)
}

func (s *sameRoundGoalService) RetargetByModel(
	_ context.Context,
	_ string,
	request protocol.RetargetGoalRequest,
) (*protocol.Goal, error) {
	s.current.Objective = request.Objective
	s.current.Metadata = map[string]any{
		protocol.GoalMetadataObjectiveRevision:     int64(2),
		protocol.GoalMetadataExecutionMode:         string(protocol.GoalExecutionModeManaged),
		protocol.GoalMetadataExecutionBindingState: string(protocol.GoalExecutionBindingStateReserved),
		protocol.GoalMetadataExecutionID:           s.successorExecutionID,
	}
	result := s.current
	return &result, nil
}

func (s *sameRoundGoalService) AuditObjectiveAlignmentByModel(
	context.Context,
	string,
	protocol.AuditGoalObjectiveAlignmentRequest,
) (*protocol.GoalObjectiveAlignmentRecord, error) {
	return nil, nil
}

func (s *sameRoundGoalService) CompleteByModel(
	context.Context,
	string,
	protocol.CompleteGoalRequest,
) (*protocol.Goal, error) {
	return nil, nil
}

func (s *sameRoundGoalService) BlockByModel(
	context.Context,
	string,
	protocol.BlockGoalRequest,
) (*protocol.Goal, error) {
	return nil, nil
}

var _ goalcontract.Service = (*sameRoundGoalService)(nil)
