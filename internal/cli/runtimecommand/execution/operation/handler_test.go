package operation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
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
	materialize   func(orchestration.ActorContext, orchestration.MaterializePlanExecutionInput) orchestration.MutationResult
	abandon       func(orchestration.AbandonExecutionInput) orchestration.MutationResult
	assign        func(orchestration.AssignWorkInput) orchestration.MutationResult
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
		&runtimecommand.CallContext{RequestID: "tool-late-submit"},
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

func TestMutationResultKeepsAuthoritativeContextWhenLarge(t *testing.T) {
	t.Parallel()

	largeContext := strings.Repeat("x", executionContextInlineLimit+1)
	result := mutationResult(orchestration.MutationResult{
		Outcome:          orchestration.MutationApplied,
		ExecutionID:      "execution-1",
		SnapshotRevision: 12,
		ExecutionContext: largeContext,
		ContextStatus:    "authoritative",
		Changed: []string{
			"assignment:assignment-1",
			"attempt:attempt-1",
		},
	})
	if result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if result.StructuredContent["outcome"] != string(orchestration.MutationApplied) ||
		result.StructuredContent["execution_id"] != "execution-1" ||
		result.StructuredContent["context_status"] != "authoritative" ||
		result.StructuredContent["execution_context"] != largeContext {
		t.Fatalf("large mutation control envelope = %#v", result.StructuredContent)
	}
	changed, ok := result.StructuredContent["changed"].([]any)
	if !ok || len(changed) != 2 {
		t.Fatalf("changed identities were externalized: %#v", result.StructuredContent["changed"])
	}
	if _, exists := result.StructuredContent["next_actions"]; exists {
		t.Fatalf("large result fabricated a recovery action: %#v", result.StructuredContent["next_actions"])
	}
}

func TestMutationResultPreservesGoalClosureNextAction(t *testing.T) {
	result := mutationResult(orchestration.MutationResult{
		Outcome: protocol.MutationResultApplied,
		NextActions: []orchestration.NextAction{{
			Domain:    "goal",
			Operation: "audit_objective_alignment",
			Reason:    "close the bound Goal",
		}},
	})
	actions, ok := result.StructuredContent["next_actions"].([]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("next_actions = %#v, want one cross-domain action", result.StructuredContent["next_actions"])
	}
	action, ok := actions[0].(map[string]any)
	if !ok || action["domain"] != "goal" || action["operation"] != "audit_objective_alignment" {
		t.Fatalf("next action = %#v, want Goal objective alignment", actions[0])
	}
}

func TestCompactRuntimeCommandContextNeverDropsResponsibility(t *testing.T) {
	t.Parallel()

	assigned := `<assigned_work><item assignment_id="assignment-1">` +
		strings.Repeat("x", executionContextInlineLimit) +
		`</item></assigned_work>`
	rendered := `<nexus_execution_context><graph_digest>` +
		strings.Repeat("g", executionContextInlineLimit) +
		`</graph_digest>` + assigned +
		`<allowed_actions><action>submit_work</action></allowed_actions></nexus_execution_context>`
	compact := compactRuntimeCommandContext(rendered)
	if strings.Contains(compact, "graph_digest") ||
		!strings.Contains(compact, `assignment_id="assignment-1"`) ||
		!strings.Contains(compact, "submit_work") {
		t.Fatalf("compact context dropped authority: %s", compact)
	}
}

func TestReviewWorkInvokeRejectsUnknownDecisionBeforeReadingExecution(t *testing.T) {
	t.Parallel()

	reviewed := false
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return executionSnapshot(3) },
		review: func(orchestration.ReviewWorkInput) orchestration.MutationResult {
			reviewed = true
			return orchestration.MutationResult{}
		},
	}
	_, err := reviewWork(svc, executionContext()).Invoke(
		context.Background(),
		map[string]any{"decision": "accept"},
		&runtimecommand.CallContext{RequestID: "review-invalid-1"},
	)
	if err == nil || !strings.Contains(err.Error(), "$.decision") {
		t.Fatalf("Invoke() error = %v, want decision enum validation", err)
	}
	if reviewed || svc.currentReads != 0 || svc.snapshotReads != 0 {
		t.Fatalf(
			"invalid review crossed the command boundary: reviewed=%t current=%d snapshot=%d",
			reviewed,
			svc.currentReads,
			svc.snapshotReads,
		)
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

func TestGetExecutionMintsExplicitRuntimeCoordinationCapability(t *testing.T) {
	snapshot := executionSnapshot(9)
	var activated bool
	var contextExecutionID string
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		contextActor: func(actor orchestration.ActorContext) {
			contextExecutionID = actor.ExecutionID
		},
		activate: func(
			actor orchestration.ActorContext,
			activatedSnapshot *protocol.ExecutionSnapshot,
		) error {
			activated = actor.AgentID == "agent-1" &&
				actor.ExecutionID == snapshot.Execution.ID &&
				activatedSnapshot == snapshot
			return nil
		},
	}
	sctx := executionContext()
	sctx.AgentID = "agent-1"
	definition := getExecution(svc, sctx)
	if _, err := definition.ContextHandler(
		context.Background(),
		map[string]any{"execution_id": snapshot.Execution.ID},
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if !activated || contextExecutionID != snapshot.Execution.ID {
		t.Fatalf(
			"get_execution binding activated=%t context execution=%q",
			activated,
			contextExecutionID,
		)
	}
}

func TestGetExecutionLetsRoomMemberObserveWithoutCoordinationCapability(t *testing.T) {
	snapshot := executionSnapshot(9)
	var activated bool
	var contextActor orchestration.ActorContext
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		contextActor: func(actor orchestration.ActorContext) {
			contextActor = actor
		},
		activate: func(
			orchestration.ActorContext,
			*protocol.ExecutionSnapshot,
		) error {
			activated = true
			return &orchestration.DomainError{
				Code:    orchestration.ErrorCodeWrongOwner,
				Message: "only the execution coordinator may perform this operation",
			}
		},
	}
	sctx := executionContext()
	sctx.AgentID = "agent-2"
	sctx.Role = orchestration.ExecutionActorMember
	result, err := getExecution(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{"execution_id": snapshot.Execution.ID},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || activated || !contextActor.ObservationOnly ||
		contextActor.ExecutionID != snapshot.Execution.ID {
		t.Fatalf(
			"member read result=%+v activated=%t actor=%+v",
			result,
			activated,
			contextActor,
		)
	}
}

func TestGetExecutionKeepsExactRoomResponsibilityView(t *testing.T) {
	t.Parallel()

	snapshot := executionSnapshot(9)
	snapshot.Plan = &protocol.ExecutionPlanRevision{ID: "plan-current"}
	for _, test := range []struct {
		name          string
		workBinding   *protocol.ExecutionWorkBinding
		reviewBinding *protocol.ExecutionReviewBinding
		observation   bool
	}{
		{
			name: "worker",
			workBinding: &protocol.ExecutionWorkBinding{
				ExecutionID: snapshot.Execution.ID,
				PlanID:      snapshot.Plan.ID,
			},
		},
		{
			name: "reviewer",
			reviewBinding: &protocol.ExecutionReviewBinding{
				ExecutionID: snapshot.Execution.ID,
				PlanID:      snapshot.Plan.ID,
			},
		},
		{
			name: "stale plan",
			workBinding: &protocol.ExecutionWorkBinding{
				ExecutionID: snapshot.Execution.ID,
				PlanID:      "plan-old",
			},
			observation: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var contextActor orchestration.ActorContext
			svc := &fakeExecutionService{
				current: func() *protocol.ExecutionSnapshot { return snapshot },
				contextActor: func(actor orchestration.ActorContext) {
					contextActor = actor
				},
			}
			sctx := executionContext()
			sctx.AgentID = "agent-2"
			sctx.Role = orchestration.ExecutionActorMember
			sctx.WorkBinding = test.workBinding
			sctx.ReviewBinding = test.reviewBinding
			result, err := getExecution(svc, sctx).ContextHandler(
				context.Background(),
				map[string]any{"execution_id": snapshot.Execution.ID},
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError || contextActor.ObservationOnly != test.observation {
				t.Fatalf("result=%+v actor=%+v, observation=%t", result, contextActor, test.observation)
			}
		})
	}
}

func TestPromoteExecutionToGoalUpgradesSharedRoundAuthority(t *testing.T) {
	current := executionSnapshot(9)
	promoted := *current
	promoted.Execution = current.Execution
	promoted.Execution.GoalID = "goal-promoted"
	promoted.Execution.GoalObjectiveRevision = 3
	promoted.Execution.Version = 10

	var contextActor orchestration.ActorContext
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return current },
		promote: func(orchestration.PromoteExecutionToGoalInput) orchestration.MutationResult {
			result := orchestration.AppliedResult(
				&promoted,
				[]string{"goal:goal-promoted"},
				nil,
			)
			result.GoalAuthority = &orchestration.GoalAuthorityReceipt{
				GoalID:            promoted.Execution.GoalID,
				ObjectiveRevision: promoted.Execution.GoalObjectiveRevision,
				ExecutionID:       promoted.Execution.ID,
			}
			return result
		},
		contextActor: func(actor orchestration.ActorContext) {
			contextActor = actor
		},
	}
	sctx := executionContext()
	sctx.GoalAuthority = runtimectx.NewGoalAuthorityState("", 0, "")
	result, err := promoteExecutionToGoal(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{"activation_reason": "substantial_complexity"},
		&runtimecommand.CallContext{RequestID: "tool-promote"},
	)
	if err != nil {
		t.Fatal(err)
	}
	authority, ok := sctx.GoalAuthority.Load()
	if result.IsError || !ok ||
		authority.GoalID != promoted.Execution.GoalID ||
		authority.ObjectiveRevision != promoted.Execution.GoalObjectiveRevision ||
		authority.ExecutionID != promoted.Execution.ID {
		t.Fatalf("result=%#v authority=%+v ok=%t", result, authority, ok)
	}
	if contextActor.GoalID != authority.GoalID ||
		contextActor.GoalObjectiveRevision != authority.ObjectiveRevision {
		t.Fatalf("fresh context actor = %+v, authority = %+v", contextActor, authority)
	}
}

func TestBindMutationGoalAuthorityRequiresConfirmedReceipt(t *testing.T) {
	snapshot := executionSnapshot(2)
	snapshot.Execution.GoalID = "goal-1"
	snapshot.Execution.GoalObjectiveRevision = 4
	for _, test := range []struct {
		name    string
		outcome orchestration.MutationOutcome
		receipt bool
		want    bool
	}{
		{name: "applied confirmed", outcome: orchestration.MutationApplied, receipt: true, want: true},
		{name: "idempotent confirmation", outcome: orchestration.MutationNoOp, receipt: true, want: true},
		{name: "applied confirmation pending", outcome: orchestration.MutationApplied, want: false},
		{name: "rejected", outcome: orchestration.MutationRejected, want: false},
		{name: "superseded", outcome: orchestration.MutationSuperseded, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			authority := runtimectx.NewGoalAuthorityState("", 0, "")
			result := orchestration.MutationResult{
				Outcome:  test.outcome,
				Snapshot: snapshot,
			}
			if test.receipt {
				result.GoalAuthority = &orchestration.GoalAuthorityReceipt{
					GoalID:            snapshot.Execution.GoalID,
					ObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
					ExecutionID:       snapshot.Execution.ID,
				}
			}
			got := bindMutationGoalAuthority(contract.Context{
				GoalAuthority: authority,
			}, result)
			if got != test.want {
				t.Fatalf("bind = %t, want %t", got, test.want)
			}
			loaded, ok := authority.Load()
			if ok != test.want {
				t.Fatalf("authority ok = %t, want %t; authority=%+v", ok, test.want, loaded)
			}
		})
	}
}

func TestExecutionCommandResultsDoNotDuplicateFullSnapshot(t *testing.T) {
	snapshot := executionSnapshot(9)
	for index := 0; index < 128; index++ {
		snapshot.CompletionBlockers = append(
			snapshot.CompletionBlockers,
			strings.Repeat("large historical blocker ", 64),
		)
	}
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		assign: func(orchestration.AssignWorkInput) orchestration.MutationResult {
			return orchestration.NoOpResult(snapshot, "captured")
		},
		context: `<nexus_execution_context execution_version="9">` +
			`<allowed_actions><action>assign_work</action></allowed_actions>` +
			`</nexus_execution_context>`,
	}

	readResult, err := getExecution(svc, executionContext()).ContextHandler(
		context.Background(),
		map[string]any{},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	mutationResult, err := assignWork(svc, executionContext()).ContextHandler(
		context.Background(),
		map[string]any{
			"logical_key":     "research",
			"target_agent_id": "agent-2",
		},
		&runtimecommand.CallContext{RequestID: "tool-compact-result"},
	)
	if err != nil {
		t.Fatal(err)
	}

	for name, result := range map[string]runtimecommand.Result{
		"get_execution": readResult,
		"mutation":      mutationResult,
	} {
		if _, duplicated := result.StructuredContent["snapshot"]; duplicated {
			t.Fatalf("%s result duplicated the full snapshot: %#v", name, result.StructuredContent)
		}
		if result.StructuredContent["execution_context"] == nil ||
			result.StructuredContent["snapshot_revision"] != float64(9) &&
				result.StructuredContent["snapshot_revision"] != int64(9) {
			t.Fatalf("%s compact result = %#v", name, result.StructuredContent)
		}
		encoded, marshalErr := json.Marshal(result.StructuredContent)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if len(encoded) > 4096 {
			t.Fatalf("%s compact result is %d bytes, want <= 4096", name, len(encoded))
		}
	}
}

func TestAssignWorkReloadsAndInjectsLatestRevision(t *testing.T) {
	snapshot := executionSnapshot(9)
	var captured orchestration.AssignWorkInput
	var contextExecutionID string
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		contextActor: func(actor orchestration.ActorContext) {
			contextExecutionID = actor.ExecutionID
		},
		assign: func(input orchestration.AssignWorkInput) orchestration.MutationResult {
			captured = input
			return orchestration.NoOpResult(snapshot, "captured")
		},
	}
	definition := assignWork(svc, executionContext())
	result, err := definition.ContextHandler(
		context.Background(),
		map[string]any{
			"logical_key":     "research",
			"target_agent_id": "agent-2",
		},
		&runtimecommand.CallContext{RequestID: "tool-assign"},
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
		result.StructuredContent["execution_context"] == nil ||
		contextExecutionID != snapshot.Execution.ID {
		t.Fatalf("mutation did not return a fresh action view: %#v", result.StructuredContent)
	}
}

func TestPreparePlanExecutionSealsCompleteDocumentAndTrustedFence(t *testing.T) {
	var capturedActor orchestration.ActorContext
	var capturedInput orchestration.PreparePlanExecutionInput
	proposal := sealedPlanProposal()
	proposal.GoalID = "goal-1"
	proposal.GoalObjectiveRevision = 7
	svc := &fakeExecutionService{
		prepare: func(
			actor orchestration.ActorContext,
			input orchestration.PreparePlanExecutionInput,
		) (*protocol.ExecutionPlanProposal, error) {
			capturedActor = actor
			capturedInput = input
			return proposal, nil
		},
	}
	sctx := executionContext()
	sctx.GoalAuthority = runtimectx.NewGoalAuthorityState("goal-1", 7, "")
	commandInput := validPreparePlanCommandInput()
	commandInput["goal_binding"] = "current"
	result, err := preparePlanExecution(svc, sctx).ContextHandler(
		context.Background(),
		commandInput,
		&runtimecommand.CallContext{RequestID: "tool-prepare-plan"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if capturedInput.CommandID != "tool-prepare-plan" ||
		capturedInput.PlanDocument != validPlanDocument() ||
		capturedInput.GoalBinding != orchestration.PlanGoalBindingCurrent {
		t.Fatalf("prepare input = %#v", capturedInput)
	}
	if capturedActor.OwnerUserID != sctx.OwnerUserID ||
		capturedActor.SessionKey != sctx.ScopeSessionKey ||
		capturedActor.AgentID != sctx.AgentID ||
		capturedActor.RootRoundID != sctx.RootRoundID ||
		capturedActor.GoalID != "goal-1" ||
		capturedActor.GoalObjectiveRevision != 7 {
		t.Fatalf("trusted actor = %#v", capturedActor)
	}
	if svc.currentReads != 0 || svc.snapshotReads != 0 {
		t.Fatalf("command adapter read authoritative state outside service: current=%d snapshot=%d", svc.currentReads, svc.snapshotReads)
	}
	if result.StructuredContent["outcome"] != "prepared" ||
		result.StructuredContent["proposal_id"] != proposal.ID ||
		result.StructuredContent["proposal_digest"] != proposal.ContentDigest ||
		result.StructuredContent["proposal_status"] != string(protocol.ExecutionPlanProposalStatusSealed) ||
		result.StructuredContent["goal_binding"] != string(orchestration.PlanGoalBindingCurrent) ||
		result.StructuredContent["objective_source"] != "goal" ||
		result.StructuredContent["completion_criteria_source"] != "plan_document" ||
		result.StructuredContent["item_count"] != float64(2) {
		t.Fatalf("prepared result = %#v", result.StructuredContent)
	}
	actions, ok := result.StructuredContent["next_actions"].([]any)
	if !ok || len(actions) != 1 || actions[0].(map[string]any)["operation"] != "plan_execution" {
		t.Fatalf("prepare next actions = %#v", result.StructuredContent["next_actions"])
	}
}

func TestPreparePlanExecutionEndsStaleGoalAuthorityRoundWithoutRetryLoop(t *testing.T) {
	svc := &fakeExecutionService{
		prepare: func(
			orchestration.ActorContext,
			orchestration.PreparePlanExecutionInput,
		) (*protocol.ExecutionPlanProposal, error) {
			return nil, &orchestration.DomainError{
				Code:    orchestration.ErrorCodeGoalBindingConflict,
				Message: "active Goal revision changed after this physical round started",
			}
		},
	}
	sctx := executionContext()
	sctx.GoalAuthority = runtimectx.NewGoalAuthorityState("goal-1", 1, "execution-old")
	commandInput := validPreparePlanCommandInput()
	commandInput["goal_binding"] = "current"

	result, err := preparePlanExecution(svc, sctx).ContextHandler(
		context.Background(),
		commandInput,
		&runtimecommand.CallContext{RequestID: "goal-stale-plan-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		result.StructuredContent["outcome"] != string(orchestration.MutationRejected) ||
		result.StructuredContent["reason_code"] != string(orchestration.ErrorCodeGoalBindingConflict) ||
		result.StructuredContent["context_status"] != "round_refresh_required" {
		t.Fatalf("stale Goal round result = %#v", result)
	}
	if actions, exists := result.StructuredContent["next_actions"]; exists && actions != nil {
		t.Fatalf("stale physical round must not advertise same-round retry: %#v", actions)
	}
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
		&runtimecommand.CallContext{RequestID: "goal-successor-replan-1"},
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

func TestPlanProposalBoundarySourcesFollowOperationAuthority(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		operation protocol.ExecutionPlanProposalOperation
		goalID    string
		objective string
		criteria  string
	}{
		{name: "Goal-bound create", operation: protocol.ExecutionPlanProposalCreate, goalID: "goal-1", objective: "goal", criteria: "plan_document"},
		{name: "Goal-free create", operation: protocol.ExecutionPlanProposalCreate, objective: "plan_document", criteria: "plan_document"},
		{name: "replan", operation: protocol.ExecutionPlanProposalReplan, objective: "execution", criteria: "execution"},
		{name: "replace", operation: protocol.ExecutionPlanProposalReplace, objective: "plan_document", criteria: "plan_document"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			proposal := &protocol.ExecutionPlanProposal{
				GoalID: testCase.goalID,
				Document: protocol.ExecutionPlanProposalDocument{
					Operation: testCase.operation,
				},
			}
			if got := planProposalObjectiveSource(proposal); got != testCase.objective {
				t.Fatalf("objective source = %q, want %q", got, testCase.objective)
			}
			if got := planProposalCompletionCriteriaSource(proposal); got != testCase.criteria {
				t.Fatalf("criteria source = %q, want %q", got, testCase.criteria)
			}
		})
	}
}

func TestPreparePlanExecutionReturnsCompleteParserContractOnDocumentError(t *testing.T) {
	svc := &fakeExecutionService{
		prepare: func(
			orchestration.ActorContext,
			orchestration.PreparePlanExecutionInput,
		) (*protocol.ExecutionPlanProposal, error) {
			return nil, &orchestration.DomainError{
				Code:    orchestration.ErrorCodePlanDocumentInvalid,
				Message: "plan document $.items[0].dependencies at line 11, column 5: unknown field",
			}
		},
	}
	result, err := preparePlanExecution(svc, executionContext()).ContextHandler(
		context.Background(),
		validPreparePlanCommandInput(),
		&runtimecommand.CallContext{RequestID: "tool-invalid-plan"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		result.StructuredContent["outcome"] != "rejected" ||
		result.StructuredContent["reason_code"] != "plan_document_invalid" {
		t.Fatalf("document rejection = %#v", result.StructuredContent)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 4096 {
		t.Fatalf("document rejection is %d bytes, want <= 4096", len(encoded))
	}
	payload := string(encoded)
	contract := orchestration.ExecutionPlanDocumentSchemaContract()
	documentContract, ok := result.StructuredContent["document_contract"].(map[string]any)
	if !ok {
		t.Fatalf("document contract = %#v", result.StructuredContent["document_contract"])
	}
	if documentContract["minimal_valid_create_example"] != contract.MinimalValidCreateExample {
		t.Fatalf("repair example = %#v", documentContract["minimal_valid_create_example"])
	}
	for _, expected := range append(
		append([]string{}, contract.AllowedRootFields...),
		contract.AllowedItemFields...,
	) {
		if !strings.Contains(payload, expected) {
			t.Fatalf("document rejection missing parser field %q: %s", expected, payload)
		}
	}
	for operation, requirement := range contract.OperationRequirements {
		if !strings.Contains(payload, operation) || !strings.Contains(payload, requirement) {
			t.Fatalf("document rejection missing %s requirement %q: %s", operation, requirement, payload)
		}
	}
	for _, expected := range []string{
		"dependencies",
		"depends_on or soft_depends_on",
		"do not remove fields one by one",
	} {
		if !strings.Contains(payload, expected) {
			t.Fatalf("document rejection missing repair guidance %q: %s", expected, payload)
		}
	}
}

func TestPlanOperationsStrictlyRejectUnknownTopLevelFieldsBeforeService(t *testing.T) {
	for _, test := range []struct {
		name       string
		definition func(contract.Service, contract.Context) runtimecommand.Operation
		input      map[string]any
	}{
		{
			name: "prepare",
			definition: func(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
				return preparePlanExecution(svc, sctx)
			},
			input: map[string]any{
				"plan_document": validPlanDocument(),
				"items":         []any{},
			},
		},
		{
			name: "commit",
			definition: func(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
				return planExecution(svc, sctx)
			},
			input: map[string]any{
				"proposal_id":     "proposal-1",
				"proposal_digest": strings.Repeat("a", 64),
				"execution_id":    "model-forged-fence",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			svc := &fakeExecutionService{
				prepare: func(orchestration.ActorContext, orchestration.PreparePlanExecutionInput) (*protocol.ExecutionPlanProposal, error) {
					called = true
					return sealedPlanProposal(), nil
				},
				materialize: func(orchestration.ActorContext, orchestration.MaterializePlanExecutionInput) orchestration.MutationResult {
					called = true
					return orchestration.MutationResult{}
				},
			}
			result, err := test.definition(svc, executionContext()).ContextHandler(
				context.Background(),
				test.input,
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			if called || !result.IsError || len(result.Content) != 1 ||
				!strings.Contains(result.Content[0]["text"].(string), "unknown field") {
				t.Fatalf("strict decode result=%#v service_called=%t", result, called)
			}
		})
	}
}

func TestPlanTransportEmptyArgumentsOfferAtMostOneRetry(t *testing.T) {
	for _, test := range []struct {
		name       string
		definition func(*planTransportGuard) runtimecommand.Operation
		input      map[string]any
	}{
		{
			name: "prepare",
			definition: func(guard *planTransportGuard) runtimecommand.Operation {
				return preparePlanExecution(&fakeExecutionService{}, executionContext(), guard)
			},
			input: map[string]any{"plan_document": ""},
		},
		{
			name: "commit",
			definition: func(guard *planTransportGuard) runtimecommand.Operation {
				return planExecution(&fakeExecutionService{}, executionContext(), guard)
			},
			input: map[string]any{"proposal_id": "", "proposal_digest": ""},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			definition := test.definition(&planTransportGuard{})
			first, err := definition.ContextHandler(context.Background(), test.input, nil)
			if err != nil {
				t.Fatal(err)
			}
			second, err := definition.ContextHandler(context.Background(), test.input, nil)
			if err != nil {
				t.Fatal(err)
			}
			if first.IsError || second.IsError ||
				first.StructuredContent["outcome"] != "rejected" ||
				second.StructuredContent["outcome"] != "rejected" ||
				first.StructuredContent["next_actions"] == nil ||
				second.StructuredContent["next_actions"] != nil {
				t.Fatalf("first=%#v second=%#v", first.StructuredContent, second.StructuredContent)
			}
			if !strings.Contains(second.StructuredContent["message"].(string), "stop retrying") {
				t.Fatalf("second retry message = %#v", second.StructuredContent)
			}
		})
	}
}

func TestPlanExecutionMaterializesExactSealedReference(t *testing.T) {
	var capturedActor orchestration.ActorContext
	var capturedInput orchestration.MaterializePlanExecutionInput
	snapshot := executionSnapshot(4)
	svc := &fakeExecutionService{
		materialize: func(
			actor orchestration.ActorContext,
			input orchestration.MaterializePlanExecutionInput,
		) orchestration.MutationResult {
			capturedActor = actor
			capturedInput = input
			return orchestration.NoOpResult(snapshot, "sealed proposal already materialized")
		},
	}
	sctx := executionContext()
	sctx.ExecutionID = snapshot.Execution.ID
	input := validPlanCommitCommandInput()
	result, err := planExecution(svc, sctx).ContextHandler(context.Background(), input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		capturedInput.ProposalID != input["proposal_id"] ||
		capturedInput.ProposalDigest != input["proposal_digest"] ||
		capturedActor.ExecutionID != snapshot.Execution.ID {
		t.Fatalf("result=%#v actor=%#v input=%#v", result, capturedActor, capturedInput)
	}
	if svc.currentReads != 0 || svc.snapshotReads != 0 {
		t.Fatalf("commit must resolve the exact sealed fence inside the service: current=%d snapshot=%d", svc.currentReads, svc.snapshotReads)
	}
}

func TestPlanExecutionUpgradesGoalOnlyAuthorityAfterConfirmedMaterialization(t *testing.T) {
	snapshot := executionSnapshot(4)
	snapshot.Execution.GoalID = "goal-1"
	snapshot.Execution.GoalObjectiveRevision = 7
	result := orchestration.AppliedResult(snapshot, []string{"execution:execution-1"}, nil)
	result.GoalAuthority = &orchestration.GoalAuthorityReceipt{
		GoalID:            "goal-1",
		ObjectiveRevision: 7,
		ExecutionID:       snapshot.Execution.ID,
	}
	var contextActor orchestration.ActorContext
	svc := &fakeExecutionService{
		materialize: func(
			orchestration.ActorContext,
			orchestration.MaterializePlanExecutionInput,
		) orchestration.MutationResult {
			return result
		},
		contextActor: func(actor orchestration.ActorContext) {
			contextActor = actor
		},
	}
	sctx := executionContext()
	sctx.GoalAuthority = runtimectx.NewGoalAuthorityState("goal-1", 7, "")
	commandResult, err := planExecution(svc, sctx).ContextHandler(
		context.Background(),
		validPlanCommitCommandInput(),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	authority, ok := sctx.GoalAuthority.Load()
	if commandResult.IsError || !ok ||
		authority.ExecutionID != snapshot.Execution.ID ||
		contextActor.ExecutionID != snapshot.Execution.ID ||
		contextActor.GoalID != "goal-1" ||
		contextActor.GoalObjectiveRevision != 7 {
		t.Fatalf(
			"result=%#v authority=%+v context_actor=%+v",
			commandResult,
			authority,
			contextActor,
		)
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
	definition := resumeWork(svc, executionContext())
	result, err := definition.ContextHandler(
		context.Background(),
		map[string]any{
			"logical_key": "research",
			"resolution":  "credentials were supplied",
			"evidence":    []any{"secret store reference credential-7"},
		},
		&runtimecommand.CallContext{RequestID: "tool-resume"},
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

func TestPlanModeCanPrepareButCommitKeepsTheSealedReference(t *testing.T) {
	var preparedActor orchestration.ActorContext
	var committedActor orchestration.ActorContext
	var committedInput orchestration.MaterializePlanExecutionInput
	proposal := sealedPlanProposal()
	svc := &fakeExecutionService{
		prepare: func(
			actor orchestration.ActorContext,
			_ orchestration.PreparePlanExecutionInput,
		) (*protocol.ExecutionPlanProposal, error) {
			preparedActor = actor
			return proposal, nil
		},
		materialize: func(
			actor orchestration.ActorContext,
			input orchestration.MaterializePlanExecutionInput,
		) orchestration.MutationResult {
			committedActor = actor
			committedInput = input
			return orchestration.RejectedResult(nil, &orchestration.DomainError{
				Code:    orchestration.ErrorCodePlanMode,
				Message: "leave Plan Mode before committing the sealed proposal",
			}, []orchestration.NextAction{{
				Domain: "execution", Operation: "plan_execution",
				Reason: "leave Plan Mode, then commit the same proposal references",
			}})
		},
	}
	sctx := executionContext()
	sctx.PlanMode = true
	prepared, err := preparePlanExecution(svc, sctx).ContextHandler(
		context.Background(),
		validPreparePlanCommandInput(),
		&runtimecommand.CallContext{RequestID: "tool-plan-mode-prepare"},
	)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := planExecution(svc, sctx).ContextHandler(
		context.Background(),
		validPlanCommitCommandInput(),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.IsError || prepared.StructuredContent["outcome"] != "prepared" || !preparedActor.PlanMode {
		t.Fatalf("Plan Mode prepare = %#v actor=%#v", prepared, preparedActor)
	}
	preparedActions, ok := prepared.StructuredContent["next_actions"].([]any)
	if !ok || len(preparedActions) != 1 {
		t.Fatalf("Plan Mode prepare lost commit guidance: %#v", prepared.StructuredContent)
	}
	preparedAction, ok := preparedActions[0].(map[string]any)
	preparedReason, reasonOK := preparedAction["reason"].(string)
	if !ok || !reasonOK || !strings.Contains(preparedReason, "leave Plan Mode") {
		t.Fatalf("Plan Mode prepare advertised an immediately callable commit: %#v", prepared.StructuredContent)
	}
	if committed.IsError || committed.StructuredContent["outcome"] != "rejected" ||
		committed.StructuredContent["reason_code"] != string(orchestration.ErrorCodePlanMode) ||
		!committedActor.PlanMode ||
		committedInput.ProposalID != proposal.ID ||
		committedInput.ProposalDigest != proposal.ContentDigest {
		t.Fatalf("Plan Mode commit=%#v actor=%#v input=%#v", committed, committedActor, committedInput)
	}
	if committed.StructuredContent["next_actions"] == nil {
		t.Fatalf("Plan Mode commit lost same-proposal guidance: %#v", committed.StructuredContent)
	}
}

func TestPlanExecutionRefreshesSuccessorContextWithoutOldBindings(t *testing.T) {
	old := executionSnapshot(8)
	old.Execution.Objective = "Old objective"
	old.Execution.CompletionCriteria = []string{"old accepted"}
	successor := executionSnapshot(1)
	successor.Execution.ID = "execution-successor"
	successor.Execution.Objective = "New objective"
	successor.Execution.ReplacesExecutionID = old.Execution.ID
	var committed orchestration.MaterializePlanExecutionInput
	var contextActor orchestration.ActorContext
	svc := &fakeExecutionService{
		materialize: func(
			_ orchestration.ActorContext,
			input orchestration.MaterializePlanExecutionInput,
		) orchestration.MutationResult {
			committed = input
			return orchestration.AppliedResult(successor, []string{"execution:execution-successor"}, nil)
		},
		contextActor: func(actor orchestration.ActorContext) {
			contextActor = actor
		},
	}
	sctx := executionContext()
	sctx.ExecutionID = old.Execution.ID
	sctx.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: old.Execution.ID}
	sctx.ReviewBinding = &protocol.ExecutionReviewBinding{ExecutionID: old.Execution.ID}
	input := validPlanCommitCommandInput()
	result, err := planExecution(svc, sctx).ContextHandler(
		context.Background(),
		input,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		committed.ProposalID != input["proposal_id"] ||
		committed.ProposalDigest != input["proposal_digest"] {
		t.Fatalf("result=%#v commit=%#v", result, committed)
	}
	if contextActor.ExecutionID != successor.Execution.ID ||
		contextActor.WorkBinding != nil ||
		contextActor.ReviewBinding != nil {
		t.Fatalf("successor context did not bind the replacement: %#v", contextActor)
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
		&runtimecommand.CallContext{RequestID: "tool-abandon"},
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
	definition := assignWork(svc, executionContext())
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
	definition := assignWork(svc, executionContext())
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

func TestAuditExecutionAlignmentPassesTypedReportAndLeavesRoutingToAgent(t *testing.T) {
	snapshot := executionSnapshot(9)
	snapshot.Execution.Objective = "Ship the verified report"
	snapshot.Execution.CompletionCriteria = []string{"report is verified"}
	var captured orchestration.AuditExecutionAlignmentInput
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return snapshot },
		alignment: func(input orchestration.AuditExecutionAlignmentInput) orchestration.MutationResult {
			captured = input
			result := orchestration.AppliedResult(snapshot, []string{"runtime_gate:gate-1"}, nil)
			result.Message = "Agent retains control"
			return result
		},
	}
	result, err := auditExecutionAlignment(svc, executionContext()).ContextHandler(
		context.Background(),
		map[string]any{
			"decision": "not_aligned",
			"criteria_results": []any{map[string]any{
				"criterion": "report is verified",
				"status":    "unsatisfied",
				"gap":       "verification has not run",
			}},
			"summary": "The report still needs verification.",
		},
		&runtimecommand.CallContext{RequestID: "tool-alignment"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError ||
		captured.ExecutionID != snapshot.Execution.ID ||
		captured.SnapshotRevision != 9 ||
		captured.CommandID != "tool-alignment" ||
		captured.Report.Decision != protocol.ObjectiveAlignmentNotAligned ||
		len(captured.Report.CriteriaResults) != 1 {
		t.Fatalf("result=%+v captured=%+v", result, captured)
	}
	if !strings.Contains(
		auditExecutionAlignment(nil, contract.Context{}).Description,
		"never transitions the Execution",
	) {
		t.Fatal("alignment tool description must keep the atomic non-transition contract")
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
