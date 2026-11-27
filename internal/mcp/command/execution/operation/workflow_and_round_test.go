// workflow_and_round_test.go 覆盖执行工作流与轮次职责测试。
package operation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	goalcontract "github.com/nexus-research-lab/nexus/internal/mcp/command/goal/contract"
	goaloperation "github.com/nexus-research-lab/nexus/internal/mcp/command/goal/operation"
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
	sctx := executionContext()
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
	goalOperations := goaloperation.BuildAll(goalService, goalcontract.Context{
		CurrentSessionKey:       "room-session-1",
		CurrentRoundID:          "round-1",
		CurrentAgentID:          "agent-1",
		GoalAuthority:           goalState,
		ResponsibilityAuthority: authority,
	})
	retargetOperation, ok := command.FindOperation(goalOperations, "retarget_goal")
	if !ok {
		t.Fatal("retarget_goal operation missing")
	}
	retargetResult, err := retargetOperation.Invoke(
		context.Background(),
		map[string]any{"objective": "New objective"},
		nil,
	)
	if err != nil {
		t.Fatalf("retarget command err=%v", err)
	}
	if retargetResult.IsError || fmt.Sprint(retargetResult.StructuredContent["outcome"]) != "applied" {
		t.Fatalf("retarget result=%#v", retargetResult)
	}
	predecessor.Execution.Status = protocol.ExecutionStatusSuperseded
	current = predecessor
	if actor := sctx.Actor(); actor.ExecutionID != "" || actor.ReviewBinding != nil ||
		actor.GoalObjectiveRevision != 2 {
		t.Fatalf("after retarget actor = %#v, want successor planning authority", actor)
	}

	planInput := validPreparePlanCommandInput()
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
		context.Background(), validPlanCommitCommandInput(), nil,
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

type workflowCommandService struct {
	owner   string
	request protocol.SaveWorkGraphWorkflowRequest
}

type workflowEditorCommandService struct {
	active     bool
	owner      string
	sessionKey string
	request    protocol.ReviseWorkGraphWorkflowPreviewRequest
}

func (s *workflowEditorCommandService) RuntimeEditorActive(_ context.Context, owner, sessionKey string) bool {
	return s.active && owner == "owner-a" && sessionKey == "editor-session-a"
}

func (s *workflowEditorCommandService) ReviseEditorPreview(
	_ context.Context,
	owner string,
	sessionKey string,
	request protocol.ReviseWorkGraphWorkflowPreviewRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	s.owner = owner
	s.sessionKey = sessionKey
	s.request = request
	return &protocol.WorkGraphWorkflowEditorSession{
		EditorID: "editor-a",
		Revision: request.Revision + 1,
		Preview: protocol.WorkGraphWorkflowPreview{
			Title: request.Title, Nodes: request.Nodes, Dependencies: request.Dependencies,
		},
	}, nil
}

func (s *workflowEditorCommandService) SelectEditorVersionBySession(
	_ context.Context,
	owner string,
	sessionKey string,
	headRevision int64,
	selectedRevision int64,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	s.owner = owner
	s.sessionKey = sessionKey
	return &protocol.WorkGraphWorkflowEditorSession{
		EditorID: "editor-a", Revision: headRevision,
		SelectedRevision: selectedRevision,
	}, nil
}

func (s *workflowCommandService) SavePreview(
	_ context.Context,
	owner string,
	request protocol.SaveWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflow, error) {
	s.owner = owner
	s.request = request
	return &protocol.WorkGraphWorkflow{
		ID: "workflow-a", SlashName: "deep-research",
		Nodes: []protocol.WorkGraphWorkflowNode{{LogicalKey: "research"}},
	}, nil
}

func TestDistillWorkGraphUsesTrustedOwnerSessionAndExactPreview(t *testing.T) {
	service := &workflowCommandService{}
	definition := distillWorkGraphWorkflow(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "session-a",
		RootRoundID: "round-a", RuntimeRoundID: "round-a", AgentID: "agent-a",
		WorkGraphPreviewID: "workgraph_preview_a",
	})
	result, err := definition.Invoke(context.Background(), map[string]any{
		"preview_id": "workgraph_preview_a",
	}, &command.CallContext{RequestID: "workflow-request-1"})
	if err != nil || result.IsError {
		t.Fatalf("invoke result = %#v, err=%v", result, err)
	}
	if service.owner != "owner-a" || service.request.SourceSessionKey != "session-a" || service.request.CommandID != "workflow-request-1" || service.request.PreviewID != "workgraph_preview_a" {
		t.Fatalf("trusted request = owner %q, %#v", service.owner, service.request)
	}
	if result.StructuredContent["command"] != "/deep-research" || result.StructuredContent["outcome"] != "applied" {
		t.Fatalf("result = %#v", result.StructuredContent)
	}
	if !strings.Contains(definition.Description, "原样持久化") ||
		strings.Contains(definition.Description, "Persist the exact") ||
		result.StructuredContent["message"] != "WorkGraph 命令已保存，可以在其他会话中复用。" {
		t.Fatalf("non-Chinese workflow contract or receipt: description=%q result=%#v", definition.Description, result.StructuredContent)
	}
}

func TestDistillWorkGraphRejectsPreviewOutsideHostBinding(t *testing.T) {
	service := &workflowCommandService{}
	operations := BuildWorkGraphDistillation(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "session-a",
		RuntimeSessionKey: "isolated-save-session-a",
		RootRoundID:       "round-a", RuntimeRoundID: "round-a", AgentID: "agent-a",
		WorkGraphPreviewID: "workgraph_preview_a",
	})
	if len(operations) != 1 || operations[0].Name != "distill_workgraph" {
		t.Fatalf("distillation operations = %#v", operations)
	}
	result, err := operations[0].Invoke(context.Background(), map[string]any{
		"preview_id": "workgraph_preview_b",
	}, &command.CallContext{RequestID: "workflow-request-1"})
	if err != nil || !result.IsError || service.request.PreviewID != "" {
		t.Fatalf("mismatched preview result = %#v, request=%#v, err=%v", result, service.request, err)
	}
}

func TestReviseWorkGraphPreviewUsesOnlyTrustedEditorIdentity(t *testing.T) {
	service := &workflowEditorCommandService{active: true}
	operations := BuildWorkGraphEditor(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "source-session-a",
		RuntimeSessionKey: "editor-session-a", RootRoundID: "round-a",
	})
	if len(operations) != 2 || operations[0].Name != "revise_workgraph_preview" ||
		operations[1].Name != "select_workgraph_preview_revision" {
		t.Fatalf("editor operations = %#v", operations)
	}
	input := map[string]any{
		"revision": float64(3), "slash_name": "review", "title": "复核流程",
		"description": "先产出，再复核。", "objective": "形成经过复核的交付。",
		"nodes": []any{map[string]any{
			"logical_key": "report", "role": "key", "kind": "integrate",
			"subject": "整合报告", "objective": "汇总内容", "deliverable": "报告",
			"required": true, "terminal": true, "position": float64(0),
		}},
		"dependencies": []any{},
	}
	result, err := operations[0].Invoke(
		context.Background(), input, &command.CallContext{RequestID: "editor-request-1"},
	)
	if err != nil || result.IsError {
		t.Fatalf("invoke result = %#v, err=%v", result, err)
	}
	if service.owner != "owner-a" || service.sessionKey != "editor-session-a" ||
		service.request.Revision != 3 || service.request.SlashName != "review" {
		t.Fatalf("trusted editor mutation = owner %q session %q request %#v", service.owner, service.sessionKey, service.request)
	}
	if result.StructuredContent["outcome"] != "applied" ||
		result.StructuredContent["revision"] != float64(4) {
		t.Fatalf("result = %#v", result.StructuredContent)
	}
}

type workflowAuthoringCommandService struct {
	owner      string
	sessionKey string
	execution  string
}

func (s *workflowAuthoringCommandService) InspectLibrary(_ context.Context, owner, session string) (*protocol.WorkGraphWorkflowLibrary, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowLibrary{}, nil
}

func (s *workflowAuthoringCommandService) PreviewFromExecution(_ context.Context, owner string, request protocol.PreviewWorkGraphWorkflowRequest) (*protocol.WorkGraphWorkflowPreview, error) {
	s.owner, s.sessionKey, s.execution = owner, request.SourceSessionKey, request.SourceExecutionID
	return &protocol.WorkGraphWorkflowPreview{PreviewID: "preview-a", SourceExecutionID: request.SourceExecutionID}, nil
}

func (s *workflowAuthoringCommandService) GetDraft(_ context.Context, owner, session, previewID string) (*protocol.WorkGraphWorkflowDraft, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowDraft{PreviewID: previewID, HeadRevision: 1, SelectedRevision: 1}, nil
}

func (s *workflowAuthoringCommandService) ReviseDraftPreview(_ context.Context, owner, session string, request protocol.ReviseWorkGraphWorkflowDraftRequest) (*protocol.WorkGraphWorkflowDraft, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowDraft{PreviewID: request.PreviewID, HeadRevision: request.Revision + 1, SelectedRevision: request.Revision + 1}, nil
}

func (s *workflowAuthoringCommandService) SelectDraftRevision(_ context.Context, owner, session, previewID string, head, selected int64) (*protocol.WorkGraphWorkflowDraft, error) {
	s.owner, s.sessionKey = owner, session
	return &protocol.WorkGraphWorkflowDraft{PreviewID: previewID, HeadRevision: head, SelectedRevision: selected}, nil
}

func (s *workflowAuthoringCommandService) SavePreview(_ context.Context, owner string, request protocol.SaveWorkGraphWorkflowRequest) (*protocol.WorkGraphWorkflow, error) {
	s.owner, s.sessionKey = owner, request.SourceSessionKey
	return &protocol.WorkGraphWorkflow{ID: "workflow-a", SlashName: "research"}, nil
}

func TestWorkGraphAuthoringOperationsUseTrustedSessionWithoutActiveExecution(t *testing.T) {
	service := &workflowAuthoringCommandService{}
	operations := BuildWorkGraphAuthoring(service, contract.Context{
		OwnerUserID: "owner-a", ScopeSessionKey: "session-a",
		RuntimeSessionKey: "runtime-a", RootRoundID: "round-a",
	})
	want := []string{
		"inspect_workgraph_library", "extract_workgraph_preview", "get_workgraph_preview",
		"revise_workgraph_preview", "select_workgraph_preview_revision", "save_workgraph_preview",
	}
	if len(operations) != len(want) {
		t.Fatalf("operations = %#v", operations)
	}
	for index, name := range want {
		if operations[index].Name != name {
			t.Fatalf("operation[%d] = %q, want %q", index, operations[index].Name, name)
		}
	}
	result, err := operations[1].Invoke(context.Background(), map[string]any{
		"source_execution_id": "execution-a", "output_language": "zh",
	}, nil)
	if err != nil || result.IsError || service.owner != "owner-a" ||
		service.sessionKey != "session-a" || service.execution != "execution-a" {
		t.Fatalf("extract = %#v, service=%#v, err=%v", result, service, err)
	}
}
