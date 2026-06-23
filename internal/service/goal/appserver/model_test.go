package appserver

import (
	"encoding/json"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestThreadGoalSetParamsUseCodexCamelCase(t *testing.T) {
	var params ThreadGoalSetParams
	if err := json.Unmarshal([]byte(`{"threadId":"agent:nexus:ws:dm:chat","status":"usageLimited","tokenBudget":null}`), &params); err != nil {
		t.Fatalf("unmarshal thread goal params: %v", err)
	}
	if params.ThreadID != "agent:nexus:ws:dm:chat" {
		t.Fatalf("ThreadID = %q, want camelCase threadId", params.ThreadID)
	}
	if params.Status == nil || *params.Status != ThreadGoalStatusUsageLimited {
		t.Fatalf("Status = %#v, want usageLimited", params.Status)
	}
	if !params.TokenBudget.Present || params.TokenBudget.Value != nil {
		t.Fatalf("TokenBudget = %+v, want present null", params.TokenBudget)
	}
}

func TestAppServerRPCRequestIDPreservesStringAndInteger(t *testing.T) {
	for _, input := range []string{
		`{"id":7,"method":"thread/goal/get"}`,
		`{"id":"goal-get","method":"thread/goal/get"}`,
	} {
		var request AppServerJSONRPCRequest
		if err := json.Unmarshal([]byte(input), &request); err != nil {
			t.Fatalf("unmarshal %s: %v", input, err)
		}
		output, err := json.Marshal(AppServerJSONRPCResponse{ID: request.ID, Result: map[string]any{"ok": true}})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		var roundtrip map[string]any
		if err := json.Unmarshal(output, &roundtrip); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if _, ok := roundtrip["id"]; !ok {
			t.Fatalf("response missing id: %s", string(output))
		}
	}

	var invalid AppServerJSONRPCRequest
	if err := json.Unmarshal([]byte(`{"id":1.5,"method":"thread/goal/get"}`), &invalid); err == nil {
		t.Fatal("fractional request id should be rejected")
	}
}

func TestThreadGoalFromGoalUsesCodexProjection(t *testing.T) {
	budget := int64(100)
	item := protocol.Goal{
		SessionKey:      "agent:nexus:ws:dm:chat",
		Objective:       "Ship parity",
		Status:          protocol.GoalStatusBudgetLimited,
		TokenBudget:     &budget,
		Usage:           protocol.GoalUsage{InputTokens: 20, OutputTokens: 5, TotalTokens: 25},
		TimeUsedSeconds: 7,
	}

	projected := ThreadGoalFromGoal(item)
	if projected.ThreadID != item.SessionKey ||
		projected.Status != ThreadGoalStatusBudgetLimited ||
		projected.TokenBudget == nil ||
		*projected.TokenBudget != budget ||
		projected.TokensUsed != 25 ||
		projected.TimeUsedSeconds != 7 {
		t.Fatalf("ThreadGoalFromGoal() = %#v", projected)
	}
}

func TestThreadGoalIncludesNullTokenBudget(t *testing.T) {
	output, err := json.Marshal(ThreadGoal{
		ThreadID: "agent:nexus:ws:dm:chat",
		Status:   ThreadGoalStatusActive,
	})
	if err != nil {
		t.Fatalf("marshal thread goal: %v", err)
	}
	var projected map[string]any
	if err := json.Unmarshal(output, &projected); err != nil {
		t.Fatalf("unmarshal thread goal: %v", err)
	}
	value, ok := projected["tokenBudget"]
	if !ok || value != nil {
		t.Fatalf("ThreadGoal JSON = %s, want tokenBudget:null", string(output))
	}
}

func TestThreadGoalUpdatedNotificationIncludesNullTurnID(t *testing.T) {
	output, err := json.Marshal(ThreadGoalUpdatedNotification{
		ThreadID: "agent:nexus:ws:dm:chat",
		Goal: ThreadGoal{
			ThreadID: "agent:nexus:ws:dm:chat",
			Status:   ThreadGoalStatusActive,
		},
	})
	if err != nil {
		t.Fatalf("marshal thread goal notification: %v", err)
	}
	var projected map[string]any
	if err := json.Unmarshal(output, &projected); err != nil {
		t.Fatalf("unmarshal thread goal notification: %v", err)
	}
	value, ok := projected["turnId"]
	if !ok || value != nil {
		t.Fatalf("ThreadGoalUpdatedNotification JSON = %s, want turnId:null", string(output))
	}
}
