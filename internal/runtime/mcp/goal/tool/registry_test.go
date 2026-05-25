package tool

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
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
	for _, want := range []string{"objective is achieved", "external unblock"} {
		if !strings.Contains(description, want) {
			t.Fatalf("status.description = %q, want %q", description, want)
		}
	}
	enum, ok := status["enum"].([]string)
	if !ok || !slices.Equal(enum, []string{"complete", "blocked"}) {
		t.Fatalf("status.enum = %#v, want [complete blocked]", status["enum"])
	}
	for _, want := range []string{"only to mark the goal achieved or blocked", "budget-limit"} {
		if !strings.Contains(tool.Description, want) {
			t.Fatalf("tool description missing %q: %s", want, tool.Description)
		}
	}
}

func TestUpdateGoalRejectsInvalidStatusBeforeLoadingCurrent(t *testing.T) {
	svc := &fakeUpdateGoalService{}
	tool := updateGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})

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
	tool := updateGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "complete"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.currentCalls != 1 || svc.completeCalls != 1 || svc.completedGoalID != "goal-1" {
		t.Fatalf("calls = current:%d complete:%d goal:%q", svc.currentCalls, svc.completeCalls, svc.completedGoalID)
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
	tool := updateGoal(svc, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})

	result, err := tool.Handler(context.Background(), map[string]any{"status": "blocked"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}
	if svc.currentCalls != 1 || svc.blockCalls != 1 || svc.blockedGoalID != "goal-1" {
		t.Fatalf("calls = current:%d block:%d goal:%q", svc.currentCalls, svc.blockCalls, svc.blockedGoalID)
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

type fakeUpdateGoalService struct {
	current         *protocol.Goal
	completed       *protocol.Goal
	blocked         *protocol.Goal
	currentCalls    int
	completeCalls   int
	blockCalls      int
	completedGoalID string
	blockedGoalID   string
}

func (s *fakeUpdateGoalService) Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (s *fakeUpdateGoalService) Current(context.Context, string) (*protocol.Goal, error) {
	s.currentCalls++
	if s.current == nil {
		return nil, errors.New("current goal not configured")
	}
	return s.current, nil
}

func (s *fakeUpdateGoalService) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return s.current, nil
}

func (s *fakeUpdateGoalService) CompleteByModel(_ context.Context, goalID string, _ protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	s.completeCalls++
	s.completedGoalID = goalID
	if s.completed == nil {
		return nil, errors.New("completed goal not configured")
	}
	return s.completed, nil
}

func (s *fakeUpdateGoalService) BlockByModel(_ context.Context, goalID string, _ protocol.BlockGoalRequest) (*protocol.Goal, error) {
	s.blockCalls++
	s.blockedGoalID = goalID
	if s.blocked == nil {
		return nil, errors.New("blocked goal not configured")
	}
	return s.blocked, nil
}
