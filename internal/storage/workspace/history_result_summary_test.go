package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeHistoryRowsPairsRoomResultsByAgentRound(t *testing.T) {
	rows := []protocol.Message{
		roomResultSummaryAssistant("assistant-a", "agent-a", "agent-round-a", "Agent A", 100),
		roomResultSummaryAssistant("assistant-b", "agent-b", "agent-round-b", "Agent B", 110),
		roomResultSummaryResult("result-b", "agent-b", "agent-round-b", "Agent B", 120),
		roomResultSummaryResult("result-a", "agent-a", "agent-round-a", "Agent A", 130),
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("两个 result 都应并回各自 assistant: got=%d rows=%+v", len(normalized), normalized)
	}
	assertAssistantResultSummary(t, normalized, "assistant-a", "result-a")
	assertAssistantResultSummary(t, normalized, "assistant-b", "result-b")
}

func TestNormalizeHistoryRowsPairsAgentRoundResultWithLegacyRoomAssistant(t *testing.T) {
	rows := []protocol.Message{
		roomResultSummaryAssistant("assistant-a", "agent-a", "", "Agent A", 100),
		roomResultSummaryAssistant("assistant-b", "agent-b", "", "Agent B", 110),
		roomResultSummaryResult("result-a", "agent-a", "agent-round-a", "Agent A", 120),
		roomResultSummaryResult("result-b", "agent-b", "agent-round-b", "Agent B", 130),
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("legacy assistant 应按 agent_id 与 result 配对: got=%d rows=%+v", len(normalized), normalized)
	}
	assertAssistantResultSummary(t, normalized, "assistant-a", "result-a")
	assertAssistantResultSummary(t, normalized, "assistant-b", "result-b")
}

func roomResultSummaryAssistant(
	messageID string,
	agentID string,
	agentRoundID string,
	text string,
	timestamp int64,
) protocol.Message {
	row := protocol.Message{
		"message_id":  messageID,
		"session_key": "room:group:conversation-1",
		"agent_id":    agentID,
		"round_id":    "round-root",
		"role":        "assistant",
		"content":     []map[string]any{{"type": "text", "text": text}},
		"model":       "test-model",
		"is_complete": true,
		"stop_reason": "end_turn",
		"timestamp":   timestamp,
	}
	if agentRoundID != "" {
		row["agent_round_id"] = agentRoundID
	}
	return row
}

func roomResultSummaryResult(
	messageID string,
	agentID string,
	agentRoundID string,
	text string,
	timestamp int64,
) protocol.Message {
	return protocol.Message{
		"message_id":     messageID,
		"session_key":    "room:group:conversation-1",
		"agent_id":       agentID,
		"round_id":       "round-root",
		"agent_round_id": agentRoundID,
		"role":           "result",
		"subtype":        "success",
		"result":         text,
		"timestamp":      timestamp,
	}
}

func assertAssistantResultSummary(
	t *testing.T,
	rows []protocol.Message,
	assistantMessageID string,
	resultMessageID string,
) {
	t.Helper()
	for _, row := range rows {
		if stringFromAny(row["message_id"]) != assistantMessageID {
			continue
		}
		summary, ok := row["result_summary"].(map[string]any)
		if !ok {
			t.Fatalf("assistant %s 未挂载 result_summary: %+v", assistantMessageID, row)
		}
		if actual := stringFromAny(summary["message_id"]); actual != resultMessageID {
			t.Fatalf("assistant %s 挂错 result: got=%q want=%q row=%+v", assistantMessageID, actual, resultMessageID, row)
		}
		return
	}
	t.Fatalf("缺少 assistant %s: %+v", assistantMessageID, rows)
}
