package tool

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
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
