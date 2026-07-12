package tool

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildAllExposesCodexGoalToolSet(t *testing.T) {
	tools := BuildAll(nil, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name)
	}

	want := []string{"get_goal", "create_goal", "update_goal"}
	if !slices.Equal(names, want) {
		t.Fatalf("tool names = %#v, want %#v", names, want)
	}
}

func TestBuildAllKeepsGoalToolsDiscoverable(t *testing.T) {
	tools := BuildAll(nil, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	for _, item := range tools {
		if item.AlwaysLoad {
			t.Fatalf("%s should stay deferred behind ToolSearch", item.Name)
		}
		if strings.TrimSpace(item.SearchHint) == "" {
			t.Fatalf("%s missing search hint", item.Name)
		}
	}
}

func TestBuildAllSchemasMarshalRequiredAsArrays(t *testing.T) {
	tools := BuildAll(nil, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	for _, item := range tools {
		encoded, err := json.Marshal(item.InputSchema)
		if err != nil {
			t.Fatalf("%s input schema is not JSON-serializable: %v", item.Name, err)
		}
		if strings.Contains(string(encoded), `"required":null`) {
			t.Fatalf("%s input schema marshaled invalid required:null: %s", item.Name, encoded)
		}

		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("%s input schema did not round-trip as JSON: %v", item.Name, err)
		}
		if _, ok := decoded["required"].([]any); !ok {
			t.Fatalf("%s required = %#v, want JSON array", item.Name, decoded["required"])
		}
	}
}

func TestUpdateGoalSchemaMatchesCodexStatusOnlyShape(t *testing.T) {
	tool := updateGoal(nil, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	properties, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want map", tool.InputSchema["properties"])
	}
	names := slices.Sorted(maps.Keys(properties))
	if !slices.Equal(names, []string{"status"}) {
		t.Fatalf("properties = %#v, want status-only schema", names)
	}
	required, ok := tool.InputSchema["required"].([]string)
	if !ok || !slices.Equal(required, []string{"status"}) {
		t.Fatalf("required = %#v, want [status]", tool.InputSchema["required"])
	}
	if tool.InputSchema["additionalProperties"] != false {
		t.Fatalf("additionalProperties = %#v, want false", tool.InputSchema["additionalProperties"])
	}
	status, ok := properties["status"].(map[string]any)
	if !ok {
		t.Fatalf("status = %#v, want map", properties["status"])
	}
	description, ok := status["description"].(string)
	if !ok {
		t.Fatalf("status.description = %#v, want string", status["description"])
	}
	for _, want := range []string{"objective is achieved", "three consecutive goal turns", "external unblock"} {
		if !strings.Contains(description, want) {
			t.Fatalf("status.description = %q, want %q", description, want)
		}
	}
	enum, ok := status["enum"].([]string)
	if !ok || !slices.Equal(enum, []string{"complete", "blocked"}) {
		t.Fatalf("status.enum = %#v, want [complete blocked]", status["enum"])
	}
	for _, want := range []string{"achieved or genuinely blocked", "three consecutive goal turns", "budget-limit, or usage-limit"} {
		if !strings.Contains(tool.Description, want) {
			t.Fatalf("tool description missing %q: %s", want, tool.Description)
		}
	}
}

func TestUpdateGoalRejectsInvalidStatusBeforeLoadingCurrent(t *testing.T) {
	svc := &fakeUpdateGoalService{}
	tool := updateGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat", CurrentRoundID: "round-1"})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "paused"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v, want MCP error", result)
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
	tool := updateGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat", CurrentRoundID: "round-1"})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "complete"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v, want MCP error", result)
	}
	text, _ := result.Content[0]["text"].(string)
	want := "cannot update goal because this thread has no goal"
	if text != want {
		t.Fatalf("error text = %q, want %q", text, want)
	}
	if svc.completeCalls != 0 || svc.blockCalls != 0 {
		t.Fatalf("calls = complete:%d block:%d, want no status updates", svc.completeCalls, svc.blockCalls)
	}
}

func TestUpdateGoalCompletesCurrentGoal(t *testing.T) {
	svc := &fakeUpdateGoalService{
		current: &protocol.Goal{ID: "goal-1", SessionKey: "agent:nexus:ws:dm:chat", Status: protocol.GoalStatusActive},
		completed: &protocol.Goal{
			ID:         "goal-1",
			SessionKey: "agent:nexus:ws:dm:chat",
			Objective:  "Complete parity",
			Status:     protocol.GoalStatusComplete,
		},
	}
	tool := updateGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat", CurrentRoundID: "round-1"})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "complete"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.currentCalls != 1 || svc.completeCalls != 1 || svc.completedGoalID != "goal-1" || svc.completedRoundID != "round-1" {
		t.Fatalf("calls = current:%d complete:%d goal:%q round:%q", svc.currentCalls, svc.completeCalls, svc.completedGoalID, svc.completedRoundID)
	}
	goal, ok := result.StructuredContent["goal"].(map[string]any)
	if !ok || goal["status"] != "complete" {
		t.Fatalf("goal payload = %#v, want complete goal", result.StructuredContent["goal"])
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
	tool := updateGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat", CurrentRoundID: "round-2"})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "blocked"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.currentCalls != 1 || svc.blockCalls != 1 || svc.blockedGoalID != "goal-1" || svc.blockedRoundID != "round-2" {
		t.Fatalf("calls = current:%d block:%d goal:%q round:%q", svc.currentCalls, svc.blockCalls, svc.blockedGoalID, svc.blockedRoundID)
	}
	goal, ok := result.StructuredContent["goal"].(map[string]any)
	if !ok || goal["status"] != "blocked" {
		t.Fatalf("goal payload = %#v, want blocked goal", result.StructuredContent["goal"])
	}
	if result.StructuredContent["completionBudgetReport"] != nil {
		t.Fatalf("completionBudgetReport = %#v, want nil for blocked", result.StructuredContent["completionBudgetReport"])
	}
}

func TestGetGoalDescriptionMatchesCodexShape(t *testing.T) {
	tool := getGoal(nil, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	for _, want := range []string{"current goal for this thread", "elapsed-time usage", "remaining token budget"} {
		if !strings.Contains(tool.Description, want) {
			t.Fatalf("get_goal description missing %q: %s", want, tool.Description)
		}
	}
	required, ok := tool.InputSchema["required"].([]string)
	if !ok || required == nil || len(required) != 0 {
		t.Fatalf("required = %#v, want empty required list", tool.InputSchema["required"])
	}
}

func TestCreateGoalSchemaMatchesCodexBudgetShape(t *testing.T) {
	tool := createGoal(nil, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	properties, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want map", tool.InputSchema["properties"])
	}
	budget, ok := properties["token_budget"].(map[string]any)
	if !ok {
		t.Fatalf("token_budget = %#v, want map", properties["token_budget"])
	}
	if budget["type"] != "integer" {
		t.Fatalf("token_budget.type = %#v, want integer", budget["type"])
	}
	objective, ok := properties["objective"].(map[string]any)
	if !ok {
		t.Fatalf("objective = %#v, want map", properties["objective"])
	}
	objectiveDescription, _ := objective["description"].(string)
	if !strings.Contains(objectiveDescription, "starts a new active goal only when no goal is currently defined") {
		t.Fatalf("objective.description = %q, want Codex create semantics", objectiveDescription)
	}
	budgetDescription, _ := budget["description"].(string)
	if !strings.Contains(budgetDescription, "Optional positive token budget for the new active goal") {
		t.Fatalf("token_budget.description = %q, want Codex budget semantics", budgetDescription)
	}
	if tool.Description != createGoalDescription {
		t.Fatalf("tool description = %q, want Codex create_goal description", tool.Description)
	}
}

func TestCreateGoalPassesCurrentRoundID(t *testing.T) {
	svc := &fakeCreateGoalService{}
	tool := createGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat", CurrentRoundID: "round-create"})

	result, err := tool.Handler(context.Background(), map[string]any{"objective": "Ship parity"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.createInput.SessionKey != "agent:nexus:ws:dm:chat" ||
		svc.createInput.CreatedBy != "model" ||
		svc.createInput.RoundID != "round-create" {
		t.Fatalf("create input = %#v, want current session and round", svc.createInput)
	}
}

func TestCreateGoalConflictUsesCodexModelMessage(t *testing.T) {
	svc := &fakeCreateGoalService{createErr: errors.New("current goal already exists")}
	tool := createGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat", CurrentRoundID: "round-create"})

	result, err := tool.Handler(context.Background(), map[string]any{"objective": "Ship parity"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v, want MCP error", result)
	}
	text, _ := result.Content[0]["text"].(string)
	if text != createGoalConflictMessage {
		t.Fatalf("error text = %q, want Codex create conflict message", text)
	}
}

func TestGetGoalReturnsNullWhenNoGoalExists(t *testing.T) {
	tool := getGoal(fakeGoalService{}, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
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
	blocked          *protocol.Goal
	currentCalls     int
	completeCalls    int
	blockCalls       int
	completedGoalID  string
	blockedGoalID    string
	completedRoundID string
	blockedRoundID   string
}

func (s *fakeUpdateGoalService) Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (s *fakeUpdateGoalService) Current(context.Context, string) (*protocol.Goal, error) {
	s.currentCalls++
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

func (s *fakeUpdateGoalService) CompleteByModel(_ context.Context, goalID string, request protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	s.completeCalls++
	s.completedGoalID = goalID
	s.completedRoundID = request.RoundID
	if s.completed == nil {
		return nil, errors.New("completed goal not configured")
	}
	return s.completed, nil
}

func (s *fakeUpdateGoalService) BlockByModel(_ context.Context, goalID string, request protocol.BlockGoalRequest) (*protocol.Goal, error) {
	s.blockCalls++
	s.blockedGoalID = goalID
	s.blockedRoundID = request.RoundID
	if s.blocked == nil {
		return nil, errors.New("blocked goal not configured")
	}
	return s.blocked, nil
}
