package dm

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
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
		service:                     &Service{goals: provider, history: history},
		workspacePath:               workspacePath,
		session:                     protocol.Session{SessionKey: sessionKey, AgentID: "agent-1"},
		sessionKey:                  sessionKey,
		roundID:                     "round-1",
		goalCompletionCandidateID:   "goal-1",
		goalCompletionAssistant:     assistant,
		goalCompletionReceiptStored: false,
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

func TestRoundRunnerDoesNotTreatBlockedGoalUpdateAsCompleted(t *testing.T) {
	receipts := runtimecommand.NewReceiptState()
	runner := &roundRunner{
		service:         &Service{goals: &fakeGoalContextProvider{}},
		goalIDForUsage:  "goal-1",
		commandReceipts: receipts,
	}
	receipts.Record(runtimecommand.Receipt{
		Domain: runtimecommand.DomainGoal, Operation: runtimecommand.GoalOperationUpdate,
		Outcome: string(protocol.MutationResultApplied), GoalID: "goal-1",
		GoalStatus: string(protocol.GoalStatusBlocked),
	})
	runner.recordGoalUsageFromAssistantMessage(goalCommandAssistantMessage(protocol.GoalStatusBlocked))
	if runner.goalCompletionCandidateID != "" {
		t.Fatalf("blocked update created completion candidate %q", runner.goalCompletionCandidateID)
	}
	receipts.Record(runtimecommand.Receipt{
		Domain: runtimecommand.DomainGoal, Operation: runtimecommand.GoalOperationUpdate,
		Outcome: string(protocol.MutationResultApplied), GoalID: "goal-1",
		GoalStatus: string(protocol.GoalStatusComplete),
	})
	runner.recordGoalUsageFromAssistantMessage(goalCommandAssistantMessage(protocol.GoalStatusComplete))
	if runner.goalCompletionCandidateID != "goal-1" {
		t.Fatalf("complete update candidate = %q, want goal-1", runner.goalCompletionCandidateID)
	}
}

func TestRoundRunnerUsesGoalIDFromCompletionCommandReceipt(t *testing.T) {
	receipts := runtimecommand.NewReceiptState()
	runner := &roundRunner{
		service:         &Service{goals: &fakeGoalContextProvider{}},
		commandReceipts: receipts,
	}
	receipts.Record(runtimecommand.Receipt{
		Domain: runtimecommand.DomainGoal, Operation: runtimecommand.GoalOperationUpdate,
		Outcome: string(protocol.MutationResultApplied), GoalID: "goal-from-receipt",
		GoalStatus: string(protocol.GoalStatusComplete),
	})
	runner.recordGoalUsageFromAssistantMessage(goalCommandAssistantMessage(protocol.GoalStatusComplete))
	if runner.goalCompletionCandidateID != "goal-from-receipt" {
		t.Fatalf("complete update candidate = %q, want exact receipt Goal ID", runner.goalCompletionCandidateID)
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
