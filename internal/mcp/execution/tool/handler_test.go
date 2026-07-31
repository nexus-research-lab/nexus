package tool

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type fakeExecutionService struct {
	current       func() *protocol.ExecutionSnapshot
	currentReads  int
	snapshotReads int
	ensure        func(orchestration.EnsureInput) orchestration.MutationResult
	plan          func(orchestration.PlanExecutionInput) orchestration.MutationResult
	abandon       func(orchestration.AbandonExecutionInput) orchestration.MutationResult
	assign        func(orchestration.AssignWorkInput) orchestration.MutationResult
	submit        func(orchestration.SubmitWorkInput) orchestration.MutationResult
	review        func(orchestration.ReviewWorkInput) orchestration.MutationResult
	block         func(orchestration.BlockWorkInput) orchestration.MutationResult
	resume        func(orchestration.ResumeWorkInput) orchestration.MutationResult
	takeover      func(orchestration.TakeOverWorkInput) orchestration.MutationResult
	promote       func(orchestration.PromoteExecutionToGoalInput) orchestration.MutationResult
	context       string
	contextActor  func(orchestration.ActorContext)
	activate      func(
		orchestration.ActorContext,
		*protocol.ExecutionSnapshot,
	) error
}

func (s *fakeExecutionService) Ensure(_ context.Context, _ orchestration.ActorContext, input orchestration.EnsureInput) (orchestration.MutationResult, error) {
	if s.ensure != nil {
		return s.ensure(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) GetCurrent(_ context.Context, _ orchestration.ActorContext) (*protocol.ExecutionSnapshot, error) {
	s.currentReads++
	if s.current == nil {
		return nil, nil
	}
	return s.current(), nil
}

func (s *fakeExecutionService) GetSnapshot(_ context.Context, _ orchestration.ActorContext, executionID string) (*protocol.ExecutionSnapshot, error) {
	s.snapshotReads++
	if s.current == nil {
		return nil, nil
	}
	snapshot := s.current()
	if snapshot == nil || snapshot.Execution.ID != executionID {
		return nil, nil
	}
	return snapshot, nil
}

func (s *fakeExecutionService) PlanExecution(_ context.Context, _ orchestration.ActorContext, input orchestration.PlanExecutionInput) (orchestration.MutationResult, error) {
	return s.plan(input), nil
}

func (s *fakeExecutionService) AbandonExecution(_ context.Context, _ orchestration.ActorContext, input orchestration.AbandonExecutionInput) (orchestration.MutationResult, error) {
	if s.abandon != nil {
		return s.abandon(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) AssignWork(_ context.Context, _ orchestration.ActorContext, input orchestration.AssignWorkInput) (orchestration.MutationResult, error) {
	return s.assign(input), nil
}

func (s *fakeExecutionService) SubmitWork(_ context.Context, _ orchestration.ActorContext, input orchestration.SubmitWorkInput) (orchestration.MutationResult, error) {
	if s.submit != nil {
		return s.submit(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) ReviewWork(_ context.Context, _ orchestration.ActorContext, input orchestration.ReviewWorkInput) (orchestration.MutationResult, error) {
	if s.review != nil {
		return s.review(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) BlockWork(_ context.Context, _ orchestration.ActorContext, input orchestration.BlockWorkInput) (orchestration.MutationResult, error) {
	if s.block != nil {
		return s.block(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) ResumeWork(_ context.Context, _ orchestration.ActorContext, input orchestration.ResumeWorkInput) (orchestration.MutationResult, error) {
	if s.resume != nil {
		return s.resume(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) TakeOverWork(_ context.Context, _ orchestration.ActorContext, input orchestration.TakeOverWorkInput) (orchestration.MutationResult, error) {
	if s.takeover != nil {
		return s.takeover(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) PromoteExecutionToGoal(_ context.Context, _ orchestration.ActorContext, input orchestration.PromoteExecutionToGoalInput) (orchestration.MutationResult, error) {
	if s.promote != nil {
		return s.promote(input), nil
	}
	return orchestration.MutationResult{}, nil
}

func (s *fakeExecutionService) RuntimeContext(
	_ context.Context,
	actor orchestration.ActorContext,
) (string, error) {
	if s.contextActor != nil {
		s.contextActor(actor)
	}
	if s.context != "" {
		return s.context, nil
	}
	return `<nexus_execution_context execution_version="9"><allowed_actions><action>assign_work</action></allowed_actions></nexus_execution_context>`, nil
}

func (s *fakeExecutionService) ActivateRuntimeCoordination(
	_ context.Context,
	actor orchestration.ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) error {
	if s.activate == nil {
		return nil
	}
	return s.activate(actor, snapshot)
}

func TestGetExecutionMintsExplicitRuntimeCoordinationCapability(t *testing.T) {
	snapshot := executionSnapshot(9)
	var activated bool
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		activate: func(
			actor orchestration.ActorContext,
			activatedSnapshot *protocol.ExecutionSnapshot,
		) error {
			activated = actor.AgentID == "agent-1" &&
				activatedSnapshot == snapshot
			return nil
		},
	}
	sctx := executionServerContext()
	sctx.AgentID = "agent-1"
	definition := getExecution(svc, sctx)
	if _, err := definition.ContextHandler(
		context.Background(),
		map[string]any{},
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if !activated {
		t.Fatal("get_execution did not mint the runtime coordination capability")
	}
}

func TestAssignWorkReloadsAndInjectsLatestRevision(t *testing.T) {
	snapshot := executionSnapshot(9)
	var captured orchestration.AssignWorkInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		assign: func(input orchestration.AssignWorkInput) orchestration.MutationResult {
			captured = input
			return orchestration.NoOpResult(snapshot, "captured")
		},
	}
	definition := assignWork(svc, executionServerContext())
	result, err := definition.ContextHandler(
		context.Background(),
		map[string]any{
			"logical_key":     "research",
			"target_agent_id": "agent-2",
		},
		&sdktool.CallContext{ToolUseID: "tool-assign"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if captured.ExecutionID != "execution-1" ||
		captured.SnapshotRevision != 9 ||
		captured.CommandID != "tool-assign" {
		t.Fatalf("captured input = %#v", captured)
	}
	if result.StructuredContent["context_status"] != "authoritative" ||
		result.StructuredContent["execution_context"] == nil {
		t.Fatalf("mutation did not return a fresh action view: %#v", result.StructuredContent)
	}
}

func TestPlanExecutionPassesAtomicInitialBoundaryWithoutEnsure(t *testing.T) {
	readCount := 0
	var planned orchestration.PlanExecutionInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot {
			readCount++
			return nil
		},
		ensure: func(input orchestration.EnsureInput) orchestration.MutationResult {
			t.Fatal("plan_execution must not split initial Execution and Plan creation through Ensure")
			return orchestration.MutationResult{}
		},
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planned = input
			return orchestration.NoOpResult(nil, "captured")
		},
	}
	definition := planExecution(svc, executionServerContext())
	result, err := definition.ContextHandler(
		context.Background(),
		validPlanToolInput(),
		&sdktool.CallContext{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if readCount != 1 {
		t.Fatalf("current snapshot reads = %d, want 1", readCount)
	}
	if planned.ExecutionID != "" ||
		planned.SnapshotRevision != 0 ||
		planned.CommandID == "" ||
		planned.Objective != "Deliver a verified report" ||
		len(planned.CompletionCriteria) != 1 ||
		len(planned.Draft.Items) != 2 ||
		len(planned.Draft.Items[0].OutputScopes) != 1 ||
		planned.Draft.Items[0].OutputScopes[0].Scope != "dir:report/research" ||
		len(planned.Draft.Items[1].DependsOn) != 1 ||
		planned.Draft.Items[1].DependsOn[0].LogicalKey != "research" ||
		planned.Draft.Items[1].DependsOn[0].Kind != protocol.WorkDependencyHard {
		t.Fatalf("planned input = %#v", planned)
	}
}

func TestPlanExecutionRejectsMalformedWorkGraphJSONBeforeService(t *testing.T) {
	planCalled := false
	svc := &fakeExecutionService{
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planCalled = true
			return orchestration.NoOpResult(nil, "unexpected")
		},
	}
	input := validPlanToolInput()
	input["work_graph_json"] = `[{"logical_key":"research","unknown":true}]`
	result, err := planExecution(svc, executionServerContext()).ContextHandler(
		context.Background(),
		input,
		&sdktool.CallContext{ToolUseID: "tool-malformed-workgraph"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if planCalled ||
		result.IsError ||
		result.StructuredContent["outcome"] != "rejected" ||
		result.StructuredContent["next_actions"] == nil {
		t.Fatalf("malformed WorkGraph result=%#v planCalled=%t", result, planCalled)
	}
}

func TestPlanExecutionWithoutExplicitIDIgnoresSupersededRoundBinding(t *testing.T) {
	var planned orchestration.PlanExecutionInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return nil },
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planned = input
			return orchestration.NoOpResult(nil, "captured successor proposal")
		},
	}
	sctx := executionServerContext()
	sctx.ExecutionID = "execution-superseded"

	result, err := planExecution(svc, sctx).ContextHandler(
		context.Background(),
		validPlanToolInput(),
		&sdktool.CallContext{ToolUseID: "tool-plan-goal-successor"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || planned.ExecutionID != "" {
		t.Fatalf("result=%#v planned=%#v", result, planned)
	}
	if svc.currentReads != 1 || svc.snapshotReads != 0 {
		t.Fatalf(
			"snapshot lookup used current=%d explicit=%d, want current=1 explicit=0",
			svc.currentReads,
			svc.snapshotReads,
		)
	}
}

func TestPlanExecutionPassesExplicitActiveWorkSupersession(t *testing.T) {
	snapshot := executionSnapshot(4)
	var planned orchestration.PlanExecutionInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planned = input
			return orchestration.NoOpResult(snapshot, "captured")
		},
	}
	input := validPlanToolInput()
	input["supersede_active_work"] = true
	definition := planExecution(svc, executionServerContext())
	result, err := definition.ContextHandler(
		context.Background(),
		input,
		&sdktool.CallContext{ToolUseID: "tool-replan"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || !planned.SupersedeActiveWork {
		t.Fatalf("result=%#v planned=%#v", result, planned)
	}
}

func TestResumeWorkPassesResolutionEvidenceAndLatestFence(t *testing.T) {
	snapshot := executionSnapshot(7)
	var resumed orchestration.ResumeWorkInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		resume: func(input orchestration.ResumeWorkInput) orchestration.MutationResult {
			resumed = input
			return orchestration.NoOpResult(snapshot, "captured")
		},
	}
	definition := resumeWork(svc, executionServerContext())
	result, err := definition.ContextHandler(
		context.Background(),
		map[string]any{
			"logical_key": "research",
			"resolution":  "credentials were supplied",
			"evidence":    []any{"secret store reference credential-7"},
		},
		&sdktool.CallContext{ToolUseID: "tool-resume"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		resumed.ExecutionID != "execution-1" ||
		resumed.SnapshotRevision != 7 ||
		resumed.CommandID != "tool-resume" ||
		resumed.Resolution != "credentials were supplied" ||
		len(resumed.Evidence) != 1 {
		t.Fatalf("result=%#v resumed=%#v", result, resumed)
	}
}

func TestPlanExecutionInPlanModeValidatesWithoutEnsuringExecution(t *testing.T) {
	readCount := 0
	var planned orchestration.PlanExecutionInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot {
			readCount++
			return nil
		},
		ensure: func(input orchestration.EnsureInput) orchestration.MutationResult {
			t.Fatal("Plan Mode must validate initial Execution and Plan in one service call")
			return orchestration.MutationResult{}
		},
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planned = input
			return orchestration.NoOpResult(nil, "proposal validated")
		},
	}
	sctx := executionServerContext()
	sctx.PlanMode = true
	definition := planExecution(svc, sctx)
	result, err := definition.ContextHandler(
		context.Background(),
		validPlanToolInput(),
		&sdktool.CallContext{ToolUseID: "tool-plan-proposal"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if readCount != 1 {
		t.Fatalf("current snapshot reads = %d, want 1", readCount)
	}
	if planned.ExecutionID != "" ||
		planned.SnapshotRevision != 0 ||
		planned.CommandID != "tool-plan-proposal" {
		t.Fatalf("planned input = %#v", planned)
	}
	if planned.Objective != "Deliver a verified report" ||
		len(planned.CompletionCriteria) != 1 {
		t.Fatalf("Execution proposal input = %#v", planned)
	}
}

func TestPlanExecutionInPlanModeReturnsStructuredCriterionRejection(t *testing.T) {
	planCalled := false
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return nil },
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planCalled = true
			if len(input.CompletionCriteria) != 0 {
				t.Fatalf("completion criteria = %#v", input.CompletionCriteria)
			}
			return orchestration.RejectedResult(nil, &orchestration.DomainError{
				Code:    orchestration.ErrorCodeCompletionCriteriaEmpty,
				Message: "at least one non-empty top-level completion criterion is required when creating an Execution",
			}, nil)
		},
	}
	sctx := executionServerContext()
	sctx.PlanMode = true
	input := validPlanToolInput()
	delete(input, "completion_criteria")
	result, err := planExecution(svc, sctx).ContextHandler(
		context.Background(),
		input,
		&sdktool.CallContext{ToolUseID: "tool-plan-invalid-proposal"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		result.StructuredContent["outcome"] != string(orchestration.MutationRejected) ||
		result.StructuredContent["reason_code"] != string(orchestration.ErrorCodeCompletionCriteriaEmpty) ||
		!planCalled {
		t.Fatalf("result=%#v plan_called=%t", result, planCalled)
	}
}

func TestPlanExecutionInitialCreationReturnsStructuredCriterionRejection(t *testing.T) {
	planCalled := false
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return nil },
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planCalled = true
			if len(input.CompletionCriteria) != 0 {
				t.Fatalf("completion criteria = %#v", input.CompletionCriteria)
			}
			return orchestration.RejectedResult(nil, &orchestration.DomainError{
				Code:    orchestration.ErrorCodeCompletionCriteriaEmpty,
				Message: "at least one non-empty top-level completion criterion is required when creating an Execution",
			}, nil)
		},
	}
	input := validPlanToolInput()
	delete(input, "completion_criteria")
	result, err := planExecution(svc, executionServerContext()).ContextHandler(
		context.Background(),
		input,
		&sdktool.CallContext{ToolUseID: "tool-plan-invalid-create"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		result.StructuredContent["outcome"] != string(orchestration.MutationRejected) ||
		result.StructuredContent["reason_code"] != string(orchestration.ErrorCodeCompletionCriteriaEmpty) ||
		!planCalled {
		t.Fatalf("result=%#v plan_called=%t", result, planCalled)
	}
}

func TestPlanExecutionExistingReplanMayOmitExecutionCriteria(t *testing.T) {
	snapshot := executionSnapshot(6)
	var planned orchestration.PlanExecutionInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		ensure: func(orchestration.EnsureInput) orchestration.MutationResult {
			t.Fatal("existing Execution replan must not call Ensure")
			return orchestration.MutationResult{}
		},
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planned = input
			return orchestration.NoOpResult(snapshot, "replan validated")
		},
	}
	input := validPlanToolInput()
	delete(input, "objective")
	delete(input, "completion_criteria")
	result, err := planExecution(svc, executionServerContext()).ContextHandler(
		context.Background(),
		input,
		&sdktool.CallContext{ToolUseID: "tool-replan-without-execution-criteria"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		planned.ExecutionID != snapshot.Execution.ID ||
		planned.SnapshotRevision != snapshot.Execution.Version {
		t.Fatalf("result=%#v planned=%#v", result, planned)
	}
}

func TestPlanExecutionReplacementInjectsOldFenceAndRefreshesSuccessorContext(t *testing.T) {
	old := executionSnapshot(8)
	old.Execution.Objective = "Old objective"
	old.Execution.CompletionCriteria = []string{"old accepted"}
	successor := executionSnapshot(1)
	successor.Execution.ID = "execution-successor"
	successor.Execution.Objective = "New objective"
	successor.Execution.ReplacesExecutionID = old.Execution.ID
	var planned orchestration.PlanExecutionInput
	var contextActor orchestration.ActorContext
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return old },
		plan: func(input orchestration.PlanExecutionInput) orchestration.MutationResult {
			planned = input
			return orchestration.AppliedResult(successor, []string{"execution:execution-successor"}, nil)
		},
		contextActor: func(actor orchestration.ActorContext) {
			contextActor = actor
		},
	}
	sctx := executionServerContext()
	sctx.ExecutionID = old.Execution.ID
	input := validPlanToolInput()
	input["execution_id"] = old.Execution.ID
	input["objective"] = "New objective"
	input["completion_criteria"] = []any{"new accepted"}
	input["replace_current_execution"] = true
	input["replacement_reason"] = "user changed objective"
	result, err := planExecution(svc, sctx).ContextHandler(
		context.Background(),
		input,
		&sdktool.CallContext{ToolUseID: "tool-replace"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		!planned.ReplaceCurrentExecution ||
		planned.ExecutionID != old.Execution.ID ||
		planned.SnapshotRevision != old.Execution.Version ||
		planned.ReplacementReason != "user changed objective" {
		t.Fatalf("result=%#v planned=%#v", result, planned)
	}
	if contextActor.ExecutionID != "" || contextActor.WorkBinding != nil {
		t.Fatalf("successor context retained old binding: %#v", contextActor)
	}
}

func TestAbandonExecutionClearsOldExplicitContextAfterSuccess(t *testing.T) {
	snapshot := executionSnapshot(5)
	var abandoned orchestration.AbandonExecutionInput
	var contextActor orchestration.ActorContext
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		abandon: func(input orchestration.AbandonExecutionInput) orchestration.MutationResult {
			abandoned = input
			terminal := *snapshot
			terminal.Execution.Status = protocol.ExecutionStatusCancelled
			terminal.Execution.Version++
			return orchestration.AppliedResult(&terminal, []string{"execution_cancelled:execution-1"}, nil)
		},
		contextActor: func(actor orchestration.ActorContext) {
			contextActor = actor
		},
	}
	sctx := executionServerContext()
	sctx.ExecutionID = snapshot.Execution.ID
	result, err := abandonExecution(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{
			"execution_id": snapshot.Execution.ID,
			"reason":       "user stopped",
		},
		&sdktool.CallContext{ToolUseID: "tool-abandon"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		abandoned.ExecutionID != snapshot.Execution.ID ||
		abandoned.SnapshotRevision != snapshot.Execution.Version ||
		abandoned.CommandID != "tool-abandon" {
		t.Fatalf("result=%#v abandoned=%#v", result, abandoned)
	}
	if contextActor.ExecutionID != "" || contextActor.WorkBinding != nil {
		t.Fatalf("abandon context retained old binding: %#v", contextActor)
	}
}

func TestRejectedMutationIsStructuredResultNotTransportError(t *testing.T) {
	snapshot := executionSnapshot(3)
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		assign: func(orchestration.AssignWorkInput) orchestration.MutationResult {
			return orchestration.RejectedResult(snapshot, errors.New("not ready"), nil)
		},
	}
	definition := assignWork(svc, executionServerContext())
	result, err := definition.ContextHandler(
		context.Background(),
		map[string]any{
			"logical_key":     "research",
			"target_agent_id": "agent-2",
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("domain rejection became transport error: %#v", result)
	}
	if result.StructuredContent["outcome"] != "rejected" {
		t.Fatalf("structured outcome = %#v", result.StructuredContent)
	}
	if len(result.Content) != 1 || result.Content[0]["type"] != "text" {
		t.Fatalf("text projection = %#v", result.Content)
	}
}

func TestStrictDecoderRejectsModelSuppliedSnapshotRevision(t *testing.T) {
	snapshot := executionSnapshot(3)
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		assign: func(orchestration.AssignWorkInput) orchestration.MutationResult {
			t.Fatal("service must not be called")
			return orchestration.MutationResult{}
		},
	}
	definition := assignWork(svc, executionServerContext())
	result, err := definition.ContextHandler(
		context.Background(),
		map[string]any{
			"logical_key":       "research",
			"target_agent_id":   "agent-2",
			"snapshot_revision": 1,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("unknown fencing field was accepted: %#v", result)
	}
}

func executionSnapshot(revision int64) *protocol.ExecutionSnapshot {
	return &protocol.ExecutionSnapshot{
		Execution: protocol.Execution{
			ID:                 "execution-1",
			OwnerUserID:        "owner-1",
			SessionKey:         "scope-session",
			ScopeKind:          protocol.ExecutionScopeRoom,
			RoomID:             "room-1",
			ConversationID:     "conversation-1",
			CoordinatorAgentID: "agent-1",
			Status:             protocol.ExecutionStatusActive,
			Version:            revision,
		},
	}
}

func executionServerContext() contract.ServerContext {
	return contract.ServerContext{
		OwnerUserID:       "owner-1",
		AgentID:           "agent-1",
		Role:              orchestration.ExecutionActorCoordinator,
		ActorKind:         protocol.ExecutionActorAgent,
		ScopeKind:         protocol.ExecutionScopeRoom,
		ScopeSessionKey:   "scope-session",
		RuntimeSessionKey: "runtime-session",
		RootRoundID:       "root-round",
		RuntimeRoundID:    "runtime-round",
		AgentRoundID:      "agent-round",
		RoomID:            "room-1",
		ConversationID:    "conversation-1",
	}
}

func validPlanToolInput() map[string]any {
	workGraph, err := json.Marshal([]any{
		map[string]any{
			"logical_key":         "research",
			"kind":                "produce",
			"subject":             "Research",
			"objective":           "Collect evidence",
			"deliverable":         "Evidence set",
			"acceptance_criteria": []any{"sources cited"},
			"required":            true,
			"terminal":            false,
			"output_scopes": []any{map[string]any{
				"scope": "dir:report/research",
				"mode":  "exclusive",
			}},
		},
		map[string]any{
			"logical_key":         "verify",
			"kind":                "verify",
			"subject":             "Verify",
			"objective":           "Verify evidence",
			"deliverable":         "Verification",
			"acceptance_criteria": []any{"all evidence checked"},
			"required":            true,
			"terminal":            true,
			"depends_on": []any{map[string]any{
				"logical_key": "research",
				"kind":        "hard",
			}},
		},
	})
	if err != nil {
		panic(err)
	}
	return map[string]any{
		"objective":           "Deliver a verified report",
		"completion_criteria": []any{"report accepted"},
		"revision_reason":     "initial graph",
		"work_graph_json":     string(workGraph),
	}
}
