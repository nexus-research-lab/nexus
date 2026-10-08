package dm

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestRoundRunnerPersistsAndSilentlyEnrichesGoalCompletionReceipt(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "agent-1")
	sessionKey := "agent:agent-1:ws:dm:goal-receipt"
	history := workspacestore.NewAgentHistoryStore(root)
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:          "goal-1",
			SessionKey:      sessionKey,
			Status:          protocol.GoalStatusComplete,
			TimeUsedSeconds: 754,
		},
	}
	assistant := protocol.Message{
		"message_id":  "assistant-final",
		"session_key": sessionKey,
		"agent_id":    "agent-1",
		"round_id":    "round-1",
		"role":        "assistant",
		"timestamp":   int64(1000),
		"content":     []map[string]any{{"type": "text", "text": "最终交付"}},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager(), History: history, Permission: permissionctx.NewContext()}},
		workspacePath:  workspacePath,
		session:        protocol.Session{SessionKey: sessionKey, AgentID: "agent-1"},
		sessionKey:     sessionKey,
		roundID:        "round-1",
		GoalRoundState: runtimehost.GoalRoundState{CompletionCandidateID: "goal-1", CompletionAssistant: assistant, CompletionReceiptStored: false},
	}

	runner.persistGoalCompletionReceipt(context.Background(), false)
	first := readGoalCompletionReceipt(t, history, workspacePath, runner.session)
	if first.TimeUsedSeconds == nil || *first.TimeUsedSeconds != 754 || first.ActualTokens != nil {
		t.Fatalf("first receipt = %+v, want duration only", first)
	}

	provider.mu.Lock()
	provider.report.UsageFinalized = true
	provider.report.Usage = protocol.GoalUsage{ActualTotalTokens: 62762, ActualTotalKnown: true}
	provider.mu.Unlock()
	runner.persistGoalCompletionReceipt(context.Background(), true)
	final := readGoalCompletionReceipt(t, history, workspacePath, runner.session)
	if final.ActualTokens == nil || *final.ActualTokens != 62762 {
		t.Fatalf("final receipt = %+v, want authoritative actual tokens", final)
	}

	runner.persistGoalCompletionReceipt(context.Background(), true)
	messages, err := history.ReadMessages(workspacePath, runner.session, nil)
	if err != nil {
		t.Fatal(err)
	}
	if countMessagesByID(messages, "assistant-final") != 1 {
		t.Fatalf("messages = %+v, want one merged final assistant", messages)
	}
}

func TestRoundRunnerUsesGoalIDFromCompletionCommandReceipt(t *testing.T) {
	receipts := nexusmcp.NewCommandReceiptState()
	runner := &roundRunner{
		service:         &Service{goals: &fakeGoalContextProvider{}, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		commandReceipts: receipts,
	}
	receipts.Record(nexusmcp.CommandReceipt{
		Domain: command.DomainGoal, Operation: command.GoalOperationUpdate,
		Outcome: string(protocol.MutationResultApplied), GoalID: "goal-from-receipt",
		GoalStatus: string(protocol.GoalStatusComplete),
	})
	runner.recordGoalUsageFromAssistantMessage(goalCommandAssistantMessage(protocol.GoalStatusComplete))
	if runner.CompletionCandidateID != "goal-from-receipt" {
		t.Fatalf("complete update candidate = %q, want exact receipt Goal ID", runner.CompletionCandidateID)
	}
}

func goalCommandAssistantMessage(status protocol.GoalStatus) protocol.Message {
	return protocol.Message{
		"message_id": "assistant-update-" + string(status),
		"role":       "assistant",
		"content":    []map[string]any{{"type": "text", "text": "Goal command completed"}},
	}
}

func readGoalCompletionReceipt(
	t *testing.T,
	history *workspacestore.AgentHistoryStore,
	workspacePath string,
	session protocol.Session,
) protocol.GoalCompletionReceipt {
	t.Helper()
	messages, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	var assistant protocol.Message
	for _, message := range messages {
		if stringValue(message["message_id"]) == "assistant-final" {
			assistant = message
			break
		}
	}
	if assistant == nil {
		t.Fatalf("final assistant missing: %+v", messages)
	}
	raw, ok := assistant[protocol.GoalCompletionReceiptField].(map[string]any)
	if !ok {
		t.Fatalf("receipt missing from history: %+v", assistant)
	}
	receipt := protocol.GoalCompletionReceipt{
		GoalID:  stringValue(raw["goal_id"]),
		RoundID: stringValue(raw["round_id"]),
	}
	if value, ok := receiptInt64(raw["time_used_seconds"]); ok {
		seconds := value
		receipt.TimeUsedSeconds = &seconds
	}
	if value, ok := receiptInt64(raw["actual_tokens"]); ok {
		tokens := value
		receipt.ActualTokens = &tokens
	}
	return receipt
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func countMessagesByID(messages []protocol.Message, messageID string) int {
	count := 0
	for _, message := range messages {
		if stringValue(message["message_id"]) == messageID {
			count++
		}
	}
	return count
}

func receiptInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
