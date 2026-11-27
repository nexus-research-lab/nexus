// handler_test.go 覆盖执行命令操作处理器、批量指派、注册表与 WorkBinding 状态测试。
package operation

import (
	"context"
	"errors"
	"strings"
	"testing"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	mcpcommand "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type fakeExecutionService struct {
	current       func() *protocol.ExecutionSnapshot
	snapshotError error
	currentReads  int
	snapshotReads int
	prepare       func(orchestration.ActorContext, orchestration.PreparePlanExecutionInput) (*protocol.ExecutionPlanProposal, error)
	resolvePlan   func(orchestration.ActorContext) (*protocol.ExecutionPlanProposal, error)
	materialize   func(orchestration.ActorContext, orchestration.MaterializePlanExecutionInput) orchestration.MutationResult
	abandon       func(orchestration.AbandonExecutionInput) orchestration.MutationResult
	assign        func(orchestration.AssignWorkInput) orchestration.MutationResult
	assignBatch   func(orchestration.AssignWorkBatchInput) orchestration.MutationResult
	submit        func(orchestration.SubmitWorkInput) orchestration.MutationResult
	review        func(orchestration.ReviewWorkInput) orchestration.MutationResult
	block         func(orchestration.BlockWorkInput) orchestration.MutationResult
	resume        func(orchestration.ResumeWorkInput) orchestration.MutationResult
	takeover      func(orchestration.TakeOverWorkInput) orchestration.MutationResult
	alignment     func(orchestration.AuditExecutionAlignmentInput) orchestration.MutationResult
	promote       func(orchestration.PromoteExecutionToGoalInput) orchestration.MutationResult
	context       string
	contextActor  func(orchestration.ActorContext)
	activate      func(
		orchestration.ActorContext,
		*protocol.ExecutionSnapshot,
	) error
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
	if s.snapshotError != nil {
		return nil, s.snapshotError
	}
	if s.current == nil {
		return nil, nil
	}
	snapshot := s.current()
	if snapshot == nil || snapshot.Execution.ID != executionID {
		return nil, nil
	}
	return snapshot, nil
}

func (s *fakeExecutionService) ReadCurrent(ctx context.Context, actor orchestration.ActorContext) (*protocol.ExecutionSnapshot, error) {
	return s.GetCurrent(ctx, actor)
}

func (s *fakeExecutionService) ReadSnapshot(ctx context.Context, actor orchestration.ActorContext, executionID string) (*protocol.ExecutionSnapshot, error) {
	return s.GetSnapshot(ctx, actor, executionID)
}

func TestSubmitWorkProjectsRetargetedPredecessorAsSuperseded(t *testing.T) {
	svc := &fakeExecutionService{snapshotError: &orchestration.DomainError{
		Code:    orchestration.ErrorCodeExecutionTerminal,
		Message: "the bound Room work was superseded; wait for a fresh Assignment",
	}}
	sctx := executionContext()
	result, err := submitWork(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{
			"execution_id":   "execution-retargeted",
			"result_summary": "late predecessor result",
		},
		&mcpcommand.CallContext{RequestID: "tool-late-submit"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, terminal predecessor must not be a transport failure", result)
	}
	if result.StructuredContent["outcome"] != string(protocol.MutationResultSuperseded) ||
		result.StructuredContent["reason_code"] != string(orchestration.ErrorCodeExecutionTerminal) {
		t.Fatalf("structured result = %#v, want superseded semantic outcome", result.StructuredContent)
	}
}

func TestCompactRuntimeCommandContextKeepsAuthorityWithoutRuntimeHistory(t *testing.T) {
	t.Parallel()

	rendered := `<nexus_execution_context execution_version="9">
  <graph_digest><nodes><node key="draft" /></nodes></graph_digest>
  <runtime_facts available="true"><recent_nodes><node id="tool-1" /></recent_nodes></runtime_facts>
  <assigned_work><item assignment_id="assignment-1"><objective>write</objective></item></assigned_work>
  <allowed_actions><action>submit_work</action></allowed_actions>
</nexus_execution_context>`
	compact := compactRuntimeCommandContext(rendered)
	if strings.Contains(compact, "runtime_facts") ||
		!strings.Contains(compact, "assignment-1") ||
		!strings.Contains(compact, "submit_work") ||
		!strings.Contains(compact, "graph_digest") {
		t.Fatalf("compact context = %s", compact)
	}

	oversized := strings.Replace(
		rendered,
		"<nodes><node key=\"draft\" /></nodes>",
		"<nodes>"+strings.Repeat("x", executionContextInlineLimit)+"</nodes>",
		1,
	)
	compact = compactRuntimeCommandContext(oversized)
	if strings.Contains(compact, "graph_digest") ||
		!strings.Contains(compact, "assignment-1") ||
		len(compact) > executionContextInlineLimit {
		t.Fatalf("oversized compact context length=%d value=%s", len(compact), compact)
	}
}

func (s *fakeExecutionService) PreparePlanExecution(
	_ context.Context,
	actor orchestration.ActorContext,
	input orchestration.PreparePlanExecutionInput,
) (*protocol.ExecutionPlanProposal, error) {
	if s.prepare == nil {
		return nil, errors.New("unexpected PreparePlanExecution call")
	}
	return s.prepare(actor, input)
}

func (s *fakeExecutionService) ResolvePlanExecutionProposal(
	_ context.Context,
	actor orchestration.ActorContext,
) (*protocol.ExecutionPlanProposal, error) {
	if s.resolvePlan != nil {
		return s.resolvePlan(actor)
	}
	return sealedPlanProposal(), nil
}

func (s *fakeExecutionService) MaterializePlanExecution(
	_ context.Context,
	actor orchestration.ActorContext,
	input orchestration.MaterializePlanExecutionInput,
) (orchestration.MutationResult, error) {
	if s.materialize == nil {
		return orchestration.MutationResult{}, errors.New("unexpected MaterializePlanExecution call")
	}
	return s.materialize(actor, input), nil
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

func (s *fakeExecutionService) AssignWorkBatch(_ context.Context, _ orchestration.ActorContext, input orchestration.AssignWorkBatchInput) (orchestration.MutationResult, error) {
	return s.assignBatch(input), nil
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

func (s *fakeExecutionService) AuditExecutionAlignment(_ context.Context, _ orchestration.ActorContext, input orchestration.AuditExecutionAlignmentInput) (orchestration.MutationResult, error) {
	if s.alignment != nil {
		return s.alignment(input), nil
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

func TestPreparePlanExecutionDirectsMissingCurrentExecutionToCreate(t *testing.T) {
	svc := &fakeExecutionService{
		prepare: func(
			orchestration.ActorContext,
			orchestration.PreparePlanExecutionInput,
		) (*protocol.ExecutionPlanProposal, error) {
			return nil, &orchestration.DomainError{
				Code:    orchestration.ErrorCodeNoCurrentExecution,
				Message: "replan requires a current Execution",
			}
		},
	}
	result, err := preparePlanExecution(svc, executionContext()).ContextHandler(
		context.Background(),
		map[string]any{
			"plan_document": strings.Replace(validPlanDocument(), "operation: create", "operation: replan", 1),
		},
		&mcpcommand.CallContext{RequestID: "goal-successor-replan-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		result.StructuredContent["outcome"] != string(orchestration.MutationRejected) ||
		result.StructuredContent["reason_code"] != string(orchestration.ErrorCodeNoCurrentExecution) {
		t.Fatalf("missing current Execution result = %#v", result)
	}
	actions, ok := result.StructuredContent["next_actions"].([]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("missing current Execution actions = %#v", result.StructuredContent["next_actions"])
	}
	reason, _ := actions[0].(map[string]any)["reason"].(string)
	if !strings.Contains(reason, "operation: create") || strings.Contains(reason, "replan") {
		t.Fatalf("repair reason = %q", reason)
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
	sctx := executionContext()
	sctx.ExecutionID = snapshot.Execution.ID
	result, err := abandonExecution(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{
			"execution_id": snapshot.Execution.ID,
			"reason":       "user stopped",
		},
		&mcpcommand.CallContext{RequestID: "tool-abandon"},
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

func executionContext() contract.Context {
	return contract.Context{
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

func validPreparePlanCommandInput() map[string]any {
	return map[string]any{"plan_document": validPlanDocument()}
}

func validPlanCommitCommandInput() map[string]any {
	proposal := sealedPlanProposal()
	return map[string]any{
		"proposal_id":     proposal.ID,
		"proposal_digest": proposal.ContentDigest,
	}
}

func validPlanDocument() string {
	return `nexus_plan: 1
operation: create
objective: Deliver a verified report
completion_criteria:
  - report accepted
items:
  - logical_key: research
    kind: produce
    subject: Research
    objective: Collect evidence
    deliverable: Evidence set
    acceptance_criteria:
      - sources cited
    required: true
    output_scopes:
      - dir:report/research
  - logical_key: verify
    kind: verify
    subject: Verify
    objective: Verify evidence
    deliverable: Verification
    acceptance_criteria:
      - all evidence checked
    required: true
    terminal: true
    depends_on:
      - research
`
}

func sealedPlanProposal() *protocol.ExecutionPlanProposal {
	return &protocol.ExecutionPlanProposal{
		ID:                 "proposal-1",
		OwnerUserID:        "owner-1",
		SessionKey:         "scope-session",
		ScopeKind:          protocol.ExecutionScopeRoom,
		RoomID:             "room-1",
		ConversationID:     "conversation-1",
		CoordinatorAgentID: "agent-1",
		RootRoundID:        "root-round",
		RuntimeRoundID:     "runtime-round",
		AgentRoundID:       "agent-round",
		ContentDigest:      strings.Repeat("a", 64),
		Status:             protocol.ExecutionPlanProposalStatusSealed,
		Version:            1,
		Document: protocol.ExecutionPlanProposalDocument{
			Version:   protocol.ExecutionPlanProposalDocumentVersion,
			Operation: protocol.ExecutionPlanProposalCreate,
			Objective: "Deliver a verified report",
			Items: []protocol.ExecutionPlanProposalItem{
				{LogicalKey: "research", Kind: protocol.WorkItemKindProduce},
				{LogicalKey: "verify", Kind: protocol.WorkItemKindVerify},
			},
		},
	}
}

func TestAssignmentBatchInvokesWithoutContractLookupAndBindsOnlyDurableReceipt(t *testing.T) {
	for _, outcome := range []orchestration.MutationOutcome{orchestration.MutationApplied, orchestration.MutationRejected} {
		t.Run(string(outcome), func(t *testing.T) {
			snapshot := executionSnapshot(9)
			binding := &protocol.ExecutionWorkBinding{
				ExecutionID: snapshot.Execution.ID, PlanID: "plan-1", WorkItemID: "work-self",
				SpecID: "spec-self", AssignmentID: "assignment-self", AttemptID: "attempt-self",
			}
			sctx := executionContext()
			sctx.ResponsibilityAuthority = runtimectx.NewResponsibilityAuthorityState(nil, snapshot.Execution.ID, nil, nil)
			calls := 0
			svc := &fakeExecutionService{
				current: func() *protocol.ExecutionSnapshot { return snapshot },
				assignBatch: func(input orchestration.AssignWorkBatchInput) orchestration.MutationResult {
					calls++
					if input.ExecutionID != snapshot.Execution.ID || input.SnapshotRevision != 9 || input.CommandID != "batch-request-001" || len(input.Assignments) != 2 || input.Assignments[0].TargetAgentID != "agent-worker" {
						t.Fatalf("batch envelope = %+v", input)
					}
					return orchestration.MutationResult{Outcome: outcome, ExecutionID: snapshot.Execution.ID, Snapshot: snapshot, WorkBinding: &orchestration.WorkBindingReceipt{Binding: binding}}
				},
			}
			result, err := mcpcommand.HandleSemantic(context.Background(), mcpcommand.Actor{}, "execution", "get_execution", BuildAll(svc, sctx), mcpcommand.Request{
				Domain: "execution", Action: "invoke", Operation: "assign_work_batch", RequestID: "batch-request-001",
				Input: map[string]any{"assignments": []any{
					map[string]any{"logical_key": "first", "target_agent_id": "agent-worker"},
					map[string]any{"logical_key": "self", "target_agent_id": "agent-1"},
				}},
			})
			if err != nil || calls != 1 {
				t.Fatalf("invoke = %#v err=%v calls=%d", result, err, calls)
			}
			actor := sctx.Actor()
			if outcome == orchestration.MutationApplied && (actor.WorkBinding == nil || *actor.WorkBinding != *binding) {
				t.Fatalf("applied batch did not bind exact self work: %+v", actor)
			}
			if outcome == orchestration.MutationRejected && actor.WorkBinding != nil {
				t.Fatal("rejected batch changed responsibility")
			}
		})
	}
}

func TestAssignmentBatchRejectsNestedIdentityAndBoundsBeforeStateReads(t *testing.T) {
	inputs := []map[string]any{
		{"assignments": []any{}},
		{"assignments": make([]any, 33)},
		{"assignments": []any{map[string]any{"target_agent_id": "worker", "execution_id": "another"}}},
		{"assignments": []any{map[string]any{"logical_key": "one"}}},
		{"assignments": []any{map[string]any{"target_agent_id": "worker", "snapshot_revision": 7}}},
	}
	for _, input := range inputs {
		svc := &fakeExecutionService{}
		_, err := assignWorkBatch(svc, executionContext()).Invoke(context.Background(), input, &mcpcommand.CallContext{RequestID: "batch-invalid"})
		if err == nil || svc.snapshotReads != 0 || svc.currentReads != 0 {
			t.Fatalf("invalid input reached state: %#v err=%v reads=%d/%d", input, err, svc.snapshotReads, svc.currentReads)
		}
	}
}

func TestPlanTransportRetryGuardSurvivesOperationRegistryRebuild(t *testing.T) {
	attempts := nexusmcp.NewCommandAttemptState()
	sctx := contract.Context{CommandAttempts: attempts}
	invokeEmptyPrepare := func() mcpcommand.Result {
		definition, ok := mcpcommand.FindOperation(BuildAll(nil, sctx), "prepare_plan_execution")
		if !ok {
			t.Fatal("prepare_plan_execution missing")
		}
		result, err := definition.ContextHandler(context.Background(), map[string]any{"plan_document": ""}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := invokeEmptyPrepare()
	second := invokeEmptyPrepare()
	if first.StructuredContent["next_actions"] == nil || second.StructuredContent["next_actions"] != nil ||
		!strings.Contains(second.StructuredContent["message"].(string), "stop retrying") {
		t.Fatalf("registry rebuild reset transport guard: first=%#v second=%#v", first, second)
	}
}

func TestReviewWorkClearReceiptReturnsRoomRoundToCoordination(t *testing.T) {
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	state := runtimectx.NewWorkBindingState(binding)
	result := orchestration.MutationResult{
		Outcome:     orchestration.MutationApplied,
		WorkBinding: &orchestration.WorkBindingReceipt{Clear: true},
	}
	if !applyMutationWorkBindingTransition(contract.Context{WorkBindingState: state}, result) {
		t.Fatal("clear receipt was not applied")
	}
	if loaded, ok := state.Load(); ok || loaded != nil {
		t.Fatalf("cleared state = %#v, %t", loaded, ok)
	}
}
