package operation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func testGoalAuthority(goalID string, revision int64) *runtimectx.GoalAuthorityState {
	return runtimectx.NewGoalAuthorityState(goalID, revision, "")
}

func TestAuditObjectiveAlignmentBindsCurrentGoalRoundAgentAndRevision(t *testing.T) {
	reportJSON := `{
		"decision":"aligned",
		"criteria_results":[{
			"criterion":"report accepted",
			"status":"satisfied",
			"evidence":[{"ref":"test://report","claim":"acceptance suite passed"}]
		}],
		"summary":"all completion criteria are proven"
	}`
	svc := &fakeUpdateGoalService{
		current: &protocol.Goal{
			ID:         "goal-1",
			SessionKey: "agent:nexus:ws:dm:chat",
			Objective:  "Ship parity",
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(7),
			},
		},
		alignmentRecord: &protocol.GoalObjectiveAlignmentRecord{
			ID:                "alignment-1",
			ObjectiveRevision: 7,
			RoundID:           "round-audit",
			AgentID:           "agent-1",
			Report: protocol.ObjectiveAlignmentReport{
				Decision: protocol.ObjectiveAlignmentAligned,
			},
		},
	}
	tool := auditObjectiveAlignment(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-audit",
		CurrentAgentID:    "agent-1",
		GoalAuthority:     testGoalAuthority("goal-1", 7),
	})

	result, err := tool.Handler(
		context.Background(),
		map[string]any{"report_json": reportJSON},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.alignmentGoalID != "goal-1" ||
		svc.alignmentRequest.RoundID != "round-audit" ||
		svc.alignmentRequest.AgentID != "agent-1" ||
		svc.alignmentRequest.ExpectedObjectiveRevision != 7 ||
		svc.alignmentRequest.Report.Decision != protocol.ObjectiveAlignmentAligned {
		t.Fatalf("alignment call = goal:%q request:%#v", svc.alignmentGoalID, svc.alignmentRequest)
	}
	nextAction, ok := result.StructuredContent["nextAction"].(map[string]any)
	if !ok || nextAction["domain"] != command.DomainGoal || nextAction["operation"] != "update_goal" || nextAction["status"] != "complete" {
		t.Fatalf("nextAction = %#v, want update_goal completion", result.StructuredContent["nextAction"])
	}
	text, _ := result.Content[0]["text"].(string)
	if !strings.Contains(text, `"objectiveAlignment"`) {
		t.Fatalf("text result omitted objective alignment record: %s", text)
	}
}

func TestAuditObjectiveAlignmentRejectsMalformedJSONBeforeService(t *testing.T) {
	for name, reportJSON := range map[string]string{
		"unknown nested field": `{
			"decision":"aligned",
			"criteria_results":[{
				"criterion":"done",
				"status":"satisfied",
				"evidence":[{"ref":"test://done","claim":"passed","unknown":true}]
			}],
			"summary":"done"
		}`,
		"trailing object": `{"decision":"aligned","criteria_results":[],"summary":"done"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &fakeUpdateGoalService{
				current: &protocol.Goal{ID: "must-not-load"},
				alignmentRecord: &protocol.GoalObjectiveAlignmentRecord{
					ID: "must-not-save",
				},
			}
			result, err := auditObjectiveAlignment(
				svc,
				contract.Context{},
			).Handler(
				context.Background(),
				map[string]any{"report_json": reportJSON},
			)
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || svc.currentCalls != 0 || svc.alignmentGoalID != "" {
				t.Fatalf(
					"result=%#v currentCalls=%d alignmentGoalID=%q",
					result,
					svc.currentCalls,
					svc.alignmentGoalID,
				)
			}
		})
	}
}

func TestRetargetGoalTrustedVisibleRoundLateBindsCurrentRevision(t *testing.T) {
	authority := runtimectx.NewGoalAuthorityState("", 0, "")
	svc := &fakeRetargetGoalService{
		current: &protocol.Goal{
			ID:         "goal-current",
			SessionKey: "agent:nexus:ws:dm:chat",
			Objective:  "Original objective",
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(3),
			},
		},
		retargeted: &protocol.Goal{
			ID:         "goal-current",
			SessionKey: "agent:nexus:ws:dm:chat",
			Objective:  "Corrected objective",
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(4),
			},
		},
	}
	result, err := retargetGoal(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-user-correction",
		CurrentAgentID:    "agent-1",
		GoalAuthority:     authority,
		AllowUserRetarget: true,
	}).Handler(context.Background(), map[string]any{"objective": "Corrected objective"})
	if err != nil || result.IsError {
		t.Fatalf("retarget result = %#v err=%v, want success", result, err)
	}
	if svc.request.ExpectedObjectiveRevision != 3 ||
		svc.request.RoundID != "round-user-correction" {
		t.Fatalf("retarget request = %#v, want exact late-bound revision", svc.request)
	}
	bound, ok := authority.Load()
	if !ok || bound.GoalID != "goal-current" || bound.ObjectiveRevision != 4 {
		t.Fatalf("authority after retarget = %#v ok=%t", bound, ok)
	}
}

func TestRetargetGoalUnboundNonUserRoundCannotLoadCurrentGoal(t *testing.T) {
	svc := &fakeRetargetGoalService{current: &protocol.Goal{
		ID:         "goal-current",
		SessionKey: "agent:nexus:ws:dm:chat",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(3),
		},
	}}
	result, err := retargetGoal(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-internal",
		GoalAuthority:     runtimectx.NewGoalAuthorityState("", 0, ""),
	}).Handler(context.Background(), map[string]any{"objective": "Forbidden objective"})
	if err != nil || !result.IsError {
		t.Fatalf("retarget result = %#v err=%v, want capability error", result, err)
	}
	if svc.sessionKey != "" || svc.request.Objective != "" {
		t.Fatalf("retarget mutation = session:%q request:%#v, want none", svc.sessionKey, svc.request)
	}
}

func TestRetargetGoalInPlanModeDoesNotMutateState(t *testing.T) {
	svc := &fakeRetargetGoalService{
		retargeted: &protocol.Goal{ID: "must-not-be-used"},
	}
	tool := retargetGoal(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-plan-mode",
		CurrentAgentID:    "agent-1",
		PlanMode:          true,
	})

	result, err := tool.Handler(
		context.Background(),
		map[string]any{"objective": "A structurally valid replacement objective"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || svc.sessionKey != "" || svc.request.Objective != "" {
		t.Fatalf("result=%#v mutation=%q %#v", result, svc.sessionKey, svc.request)
	}
}

func TestUpdateGoalKeepsInFlightRevisionAndUsesAdoptedRevisionNext(t *testing.T) {
	authority := testGoalAuthority("goal-1", 1)
	sctx := contract.Context{
		CurrentSessionKey: "room:group:chat",
		CurrentRoundID:    "round-recipient",
		CurrentAgentID:    "agent-recipient",
		GoalAuthority:     authority,
	}
	started := make(chan struct{})
	release := make(chan struct{})
	oldCall := &fakeUpdateGoalService{
		current:          &protocol.Goal{ID: "goal-1", SessionKey: "room:group:chat", Status: protocol.GoalStatusActive},
		completed:        &protocol.Goal{ID: "goal-1", SessionKey: "room:group:chat", Status: protocol.GoalStatusComplete},
		requiredRevision: 2,
		currentStarted:   started,
		currentRelease:   release,
	}
	type handlerResult struct {
		result command.Result
		err    error
	}
	resultCh := make(chan handlerResult, 1)
	go func() {
		result, err := updateGoal(oldCall, sctx).Handler(context.Background(), map[string]any{"status": "complete"})
		resultCh <- handlerResult{result: result, err: err}
	}()
	<-started
	authority.ObjectiveRevisionState().Store(2)
	close(release)
	oldResult := <-resultCh
	if oldResult.err != nil || !oldResult.result.IsError {
		t.Fatalf("old in-flight result = %#v err=%v, want revision error", oldResult.result, oldResult.err)
	}
	if got := oldCall.completedRequest.ExpectedObjectiveRevision; got != 1 {
		t.Fatalf("old in-flight expected revision = %d, want captured 1", got)
	}

	newCall := &fakeUpdateGoalService{
		current: &protocol.Goal{
			ID:         "goal-1",
			SessionKey: "room:group:chat",
			Status:     protocol.GoalStatusActive,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(2),
			},
		},
		completed:        &protocol.Goal{ID: "goal-1", SessionKey: "room:group:chat", Status: protocol.GoalStatusComplete},
		requiredRevision: 2,
	}
	newResult, err := updateGoal(newCall, sctx).Handler(context.Background(), map[string]any{"status": "complete"})
	if err != nil || newResult.IsError {
		t.Fatalf("post-adoption result = %#v err=%v, want success", newResult, err)
	}
	if got := newCall.completedRequest.ExpectedObjectiveRevision; got != 2 {
		t.Fatalf("post-adoption expected revision = %d, want 2", got)
	}
}

func TestUpdateGoalRejectsInvalidStatusBeforeLoadingCurrent(t *testing.T) {
	svc := &fakeUpdateGoalService{}
	tool := updateGoal(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-1",
		GoalAuthority:     testGoalAuthority("goal-1", 1),
	})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "paused"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v, want command error", result)
	}
	text, _ := result.Content[0]["text"].(string)
	if !strings.Contains(text, "only mark the existing goal complete or blocked") {
		t.Fatalf("error text = %q, want Codex status rejection", text)
	}
	if svc.currentCalls != 0 || svc.completeCalls != 0 {
		t.Fatalf("calls = current:%d complete:%d, want no service calls", svc.currentCalls, svc.completeCalls)
	}
}

func TestUpdateGoalNoCurrentGoalUsesCodexModelMessage(t *testing.T) {
	svc := &fakeUpdateGoalService{currentErr: errors.New("goal not found")}
	tool := updateGoal(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-1",
		GoalAuthority:     testGoalAuthority("goal-1", 1),
	})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "complete"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v, want command error", result)
	}
	text, _ := result.Content[0]["text"].(string)
	want := "cannot update goal because this thread has no current goal; do not retry update_goal—if the user explicitly requested a new Goal and its objective is execution-ready, use create_goal"
	if text != want {
		t.Fatalf("error text = %q, want %q", text, want)
	}
	if svc.completeCalls != 0 || svc.blockCalls != 0 {
		t.Fatalf("calls = complete:%d block:%d, want no status updates", svc.completeCalls, svc.blockCalls)
	}
}

func TestUpdateGoalCompletionRejectionReturnsDomainSpecificRecovery(t *testing.T) {
	for _, test := range []struct {
		name          string
		completeErr   error
		wantDomain    string
		wantOperation string
	}{
		{
			name:          "missing Goal alignment",
			completeErr:   goalsvc.ErrGoalAlignmentRefreshRequired,
			wantDomain:    command.DomainGoal,
			wantOperation: "audit_objective_alignment",
		},
		{
			name:          "Execution still active",
			completeErr:   goalsvc.ErrGoalExecutionNotReady,
			wantDomain:    command.DomainExecution,
			wantOperation: "get_execution",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := &fakeUpdateGoalService{
				current:     &protocol.Goal{ID: "goal-1", SessionKey: "agent:nexus:ws:dm:chat", Status: protocol.GoalStatusActive},
				completeErr: test.completeErr,
			}
			tool := updateGoal(svc, contract.Context{
				CurrentSessionKey: "agent:nexus:ws:dm:chat",
				CurrentRoundID:    "round-1",
				GoalAuthority:     testGoalAuthority("goal-1", 1),
			})

			result, err := tool.Handler(context.Background(), map[string]any{"status": "complete"})
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError {
				t.Fatalf("result = %#v, want recoverable command rejection", result)
			}
			nextAction, ok := result.StructuredContent["nextAction"].(map[string]any)
			if !ok || nextAction["domain"] != test.wantDomain || nextAction["operation"] != test.wantOperation {
				t.Fatalf("nextAction = %#v, want %s/%s", nextAction, test.wantDomain, test.wantOperation)
			}
		})
	}
}

func TestPausedGoalCompletionRecoveryNamesExactUIControl(t *testing.T) {
	svc := &fakeUpdateGoalService{
		current: &protocol.Goal{
			ID:         "goal-1",
			SessionKey: "agent:nexus:ws:dm:chat",
			Status:     protocol.GoalStatusPaused,
		},
		completeErr: goalsvc.ErrGoalInvalidState,
	}
	result, err := updateGoal(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-1",
		GoalAuthority:     testGoalAuthority("goal-1", 1),
	}).Handler(context.Background(), map[string]any{"status": "complete"})
	if err != nil {
		t.Fatal(err)
	}
	assertPausedGoalRecoveryText(t, result)
}

func TestPausedGoalAuditRecoveryNamesExactUIControl(t *testing.T) {
	svc := &fakeUpdateGoalService{
		current: &protocol.Goal{
			ID:         "goal-1",
			SessionKey: "agent:nexus:ws:dm:chat",
			Status:     protocol.GoalStatusPaused,
		},
		alignmentErr: goalsvc.ErrGoalInvalidState,
	}
	result, err := auditObjectiveAlignment(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-1",
		GoalAuthority:     testGoalAuthority("goal-1", 1),
	}).Handler(context.Background(), map[string]any{
		"report_json": `{"decision":"aligned","criteria_results":[],"summary":"done"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPausedGoalRecoveryText(t, result)
}

func assertPausedGoalRecoveryText(t *testing.T, result command.Result) {
	t.Helper()
	if !result.IsError {
		t.Fatalf("result = %#v, want paused Goal rejection", result)
	}
	text, _ := result.Content[0]["text"].(string)
	for _, fragment := range []string{
		"Goal status bar directly above this conversation's message composer",
		"Play control labeled 「继续」",
		"automatically schedules a new Goal continuation",
		"perform the remaining audit and completion work",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("paused recovery text %q omitted %q", text, fragment)
		}
	}
}

func TestUpdateGoalBlocksCurrentGoal(t *testing.T) {
	svc := &fakeUpdateGoalService{
		current: &protocol.Goal{ID: "goal-1", SessionKey: "agent:nexus:ws:dm:chat", Status: protocol.GoalStatusActive},
		blocked: &protocol.Goal{
			ID:         "goal-1",
			SessionKey: "agent:nexus:ws:dm:chat",
			Objective:  "Complete parity",
			Status:     protocol.GoalStatusBlocked,
		},
	}
	tool := updateGoal(svc, contract.Context{
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-2",
		CurrentAgentID:    "agent-2",
		GoalAuthority:     testGoalAuthority("goal-1", 1),
	})

	result, err := tool.Handler(context.Background(), map[string]any{
		"status":       "blocked",
		"blocker_id":   "dataset-unavailable",
		"reason":       "external dataset is unavailable",
		"needed_input": "restore dataset access",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.currentCalls != 1 || svc.blockCalls != 1 || svc.blockedGoalID != "goal-1" || svc.blockedRoundID != "round-2" {
		t.Fatalf("calls = current:%d block:%d goal:%q round:%q", svc.currentCalls, svc.blockCalls, svc.blockedGoalID, svc.blockedRoundID)
	}
	if svc.blockedRequest.AgentID != "agent-2" {
		t.Fatalf("blocked agent = %q, want agent-2", svc.blockedRequest.AgentID)
	}
	if svc.blockedRequest.BlockerID != "dataset-unavailable" ||
		svc.blockedRequest.Reason != "external dataset is unavailable" ||
		svc.blockedRequest.NeededInput != "restore dataset access" {
		t.Fatalf("blocked request = %#v, want durable recovery path", svc.blockedRequest)
	}
	goal, ok := result.StructuredContent["goal"].(map[string]any)
	if !ok || goal["status"] != "blocked" {
		t.Fatalf("goal payload = %#v, want blocked goal", result.StructuredContent["goal"])
	}
	if result.StructuredContent["completionBudgetReport"] != nil {
		t.Fatalf("completionBudgetReport = %#v, want nil for blocked", result.StructuredContent["completionBudgetReport"])
	}
}

func TestCreateGoalPassesCurrentRoundID(t *testing.T) {
	svc := &fakeCreateGoalService{}
	authority := runtimectx.NewGoalAuthorityState("", 0, "")
	tool := createGoal(svc, contract.Context{
		OwnerUserID:       "owner-1",
		CurrentSessionKey: "agent:nexus:ws:dm:chat",
		CurrentRoundID:    "round-create",
		CurrentAgentID:    "agent-1",
		GoalAuthority:     authority,
	})

	result, err := tool.Handler(context.Background(), map[string]any{"objective": "Ship parity"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.createInput.SessionKey != "agent:nexus:ws:dm:chat" ||
		svc.createInput.CreatedBy != "model" ||
		svc.createInput.RoundID != "round-create" ||
		svc.createInput.OwnerUserID != "owner-1" ||
		svc.createInput.AgentID != "agent-1" {
		t.Fatalf("create input = %#v, want current owner, session, and round", svc.createInput)
	}
	createdAuthority, ok := authority.Load()
	if !ok || createdAuthority.GoalID != "goal-1" || createdAuthority.ObjectiveRevision != 1 {
		t.Fatalf("authority after create = %#v, ok=%t", createdAuthority, ok)
	}
}

func TestCreateGoalConflictUsesCodexModelMessage(t *testing.T) {
	svc := &fakeCreateGoalService{createErr: errors.New("current goal already exists")}
	tool := createGoal(svc, contract.Context{CurrentSessionKey: "agent:nexus:ws:dm:chat", CurrentRoundID: "round-create"})

	result, err := tool.Handler(context.Background(), map[string]any{"objective": "Ship parity"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v, want command error", result)
	}
	text, _ := result.Content[0]["text"].(string)
	if text != createGoalConflictMessage {
		t.Fatalf("error text = %q, want Codex create conflict message", text)
	}
}

func TestGetGoalReturnsNullWhenNoGoalExists(t *testing.T) {
	tool := getGoal(fakeGoalService{}, contract.Context{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	result, err := tool.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want successful null goal payload", result)
	}
	if result.StructuredContent["goal"] != nil || result.StructuredContent["remainingTokens"] != nil || result.StructuredContent["completionBudgetReport"] != nil {
		t.Fatalf("structured content = %#v, want null goal, remainingTokens, and completionBudgetReport", result.StructuredContent)
	}
	text, ok := result.Content[0]["text"].(string)
	if !ok {
		t.Fatalf("text content = %#v, want string", result.Content)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("text content is not JSON: %v; text=%s", err, text)
	}
	if _, ok := decoded["remainingTokens"]; !ok {
		t.Fatalf("decoded text = %#v, want Codex-style JSON payload", decoded)
	}
}

type fakeGoalService struct{}

func (fakeGoalService) Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (fakeGoalService) Current(context.Context, string) (*protocol.Goal, error) {
	return nil, errors.New("Current should not be called by get_goal")
}

func (fakeGoalService) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (fakeGoalService) RetargetByModel(context.Context, string, protocol.RetargetGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (fakeGoalService) AuditObjectiveAlignmentByModel(context.Context, string, protocol.AuditGoalObjectiveAlignmentRequest) (*protocol.GoalObjectiveAlignmentRecord, error) {
	return nil, nil
}

func (fakeGoalService) CompleteByModel(context.Context, string, protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (fakeGoalService) BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

type fakeCreateGoalService struct {
	createInput protocol.CreateGoalRequest
	createErr   error
}

func (s *fakeCreateGoalService) Create(_ context.Context, request protocol.CreateGoalRequest) (*protocol.Goal, error) {
	s.createInput = request
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &protocol.Goal{
		ID:         "goal-1",
		SessionKey: request.SessionKey,
		Objective:  request.Objective,
		Status:     protocol.GoalStatusActive,
	}, nil
}

func (s *fakeCreateGoalService) Current(context.Context, string) (*protocol.Goal, error) {
	return nil, errors.New("Current should not be called by create_goal")
}

func (s *fakeCreateGoalService) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return nil, errors.New("CurrentOptional should not be called by create_goal")
}

func (s *fakeCreateGoalService) RetargetByModel(context.Context, string, protocol.RetargetGoalRequest) (*protocol.Goal, error) {
	return nil, errors.New("RetargetByModel should not be called by create_goal")
}

func (s *fakeCreateGoalService) AuditObjectiveAlignmentByModel(context.Context, string, protocol.AuditGoalObjectiveAlignmentRequest) (*protocol.GoalObjectiveAlignmentRecord, error) {
	return nil, errors.New("AuditObjectiveAlignmentByModel should not be called by create_goal")
}

func (s *fakeCreateGoalService) CompleteByModel(context.Context, string, protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	return nil, errors.New("CompleteByModel should not be called by create_goal")
}

func (s *fakeCreateGoalService) BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error) {
	return nil, errors.New("BlockByModel should not be called by create_goal")
}

type fakeUpdateGoalService struct {
	current          *protocol.Goal
	currentErr       error
	completed        *protocol.Goal
	completeErr      error
	blocked          *protocol.Goal
	currentCalls     int
	completeCalls    int
	blockCalls       int
	completedGoalID  string
	blockedGoalID    string
	completedRoundID string
	blockedRoundID   string
	completedRequest protocol.CompleteGoalRequest
	blockedRequest   protocol.BlockGoalRequest
	alignmentRecord  *protocol.GoalObjectiveAlignmentRecord
	alignmentErr     error
	alignmentRequest protocol.AuditGoalObjectiveAlignmentRequest
	alignmentGoalID  string
	requiredRevision int64
	currentStarted   chan<- struct{}
	currentRelease   <-chan struct{}
}

func (s *fakeUpdateGoalService) Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (s *fakeUpdateGoalService) Current(context.Context, string) (*protocol.Goal, error) {
	s.currentCalls++
	if s.currentStarted != nil {
		s.currentStarted <- struct{}{}
	}
	if s.currentRelease != nil {
		<-s.currentRelease
	}
	if s.currentErr != nil {
		return nil, s.currentErr
	}
	if s.current == nil {
		return nil, errors.New("current goal not configured")
	}
	return s.current, nil
}

func (s *fakeUpdateGoalService) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return s.current, nil
}

func (s *fakeUpdateGoalService) RetargetByModel(context.Context, string, protocol.RetargetGoalRequest) (*protocol.Goal, error) {
	return nil, errors.New("RetargetByModel should not be called by update_goal")
}

func (s *fakeUpdateGoalService) AuditObjectiveAlignmentByModel(
	_ context.Context,
	goalID string,
	request protocol.AuditGoalObjectiveAlignmentRequest,
) (*protocol.GoalObjectiveAlignmentRecord, error) {
	s.alignmentGoalID = goalID
	s.alignmentRequest = request
	if s.alignmentErr != nil {
		return nil, s.alignmentErr
	}
	if s.alignmentRecord == nil {
		return nil, errors.New("alignment record not configured")
	}
	return s.alignmentRecord, nil
}

func (s *fakeUpdateGoalService) CompleteByModel(_ context.Context, goalID string, request protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	s.completeCalls++
	s.completedGoalID = goalID
	s.completedRoundID = request.RoundID
	s.completedRequest = request
	if s.requiredRevision > 0 && request.ExpectedObjectiveRevision != s.requiredRevision {
		return nil, errors.New("goal objective changed after this tool call started")
	}
	if s.completeErr != nil {
		return nil, s.completeErr
	}
	if s.completed == nil {
		return nil, errors.New("completed goal not configured")
	}
	return s.completed, nil
}

func (s *fakeUpdateGoalService) BlockByModel(_ context.Context, goalID string, request protocol.BlockGoalRequest) (*protocol.Goal, error) {
	s.blockCalls++
	s.blockedGoalID = goalID
	s.blockedRoundID = request.RoundID
	s.blockedRequest = request
	if s.blocked == nil {
		return nil, errors.New("blocked goal not configured")
	}
	return s.blocked, nil
}

type fakeRetargetGoalService struct {
	sessionKey       string
	request          protocol.RetargetGoalRequest
	retargeted       *protocol.Goal
	current          *protocol.Goal
	completed        *protocol.Goal
	completedRequest protocol.CompleteGoalRequest
	err              error
}

func (s *fakeRetargetGoalService) Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error) {
	return nil, errors.New("Create should not be called by retarget_goal")
}

func (s *fakeRetargetGoalService) Current(context.Context, string) (*protocol.Goal, error) {
	if s.current != nil {
		return s.current, nil
	}
	return nil, errors.New("Current should not be called by retarget_goal")
}

func (s *fakeRetargetGoalService) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return nil, errors.New("CurrentOptional should not be called by retarget_goal")
}

func (s *fakeRetargetGoalService) RetargetByModel(_ context.Context, sessionKey string, request protocol.RetargetGoalRequest) (*protocol.Goal, error) {
	s.sessionKey = sessionKey
	s.request = request
	if s.err == nil && s.retargeted != nil {
		s.current = s.retargeted
	}
	return s.retargeted, s.err
}

func (s *fakeRetargetGoalService) AuditObjectiveAlignmentByModel(context.Context, string, protocol.AuditGoalObjectiveAlignmentRequest) (*protocol.GoalObjectiveAlignmentRecord, error) {
	return nil, errors.New("AuditObjectiveAlignmentByModel should not be called by retarget_goal")
}

func (s *fakeRetargetGoalService) CompleteByModel(_ context.Context, _ string, request protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	if s.completed != nil {
		s.completedRequest = request
		return s.completed, nil
	}
	return nil, errors.New("CompleteByModel should not be called by retarget_goal")
}

func (s *fakeRetargetGoalService) BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error) {
	return nil, errors.New("BlockByModel should not be called by retarget_goal")
}
