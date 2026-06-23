package workspace

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type roundTerminalStatus string

const (
	roundStatusRunning     roundTerminalStatus = "running"
	roundStatusSuccess     roundTerminalStatus = "success"
	roundStatusInterrupted roundTerminalStatus = "interrupted"
	roundStatusError       roundTerminalStatus = "error"
)

func materializeUnfinishedRounds(rows []protocol.Message, activeRoundIDs map[string]struct{}) []protocol.Message {
	if len(rows) == 0 {
		return rows
	}
	type roundSnapshot struct {
		RoundID         string
		SessionKey      string
		RoomID          string
		ConversationID  string
		AgentID         string
		SessionID       string
		ParentID        string
		LastTimestampMS int64
		HasResult       bool
		TerminalStatus  roundTerminalStatus
	}

	rounds := make(map[string]*roundSnapshot)
	for _, row := range rows {
		roundID := stringFromAny(row["round_id"])
		if roundID == "" {
			continue
		}
		snapshot := rounds[roundID]
		if snapshot == nil {
			snapshot = &roundSnapshot{
				RoundID:        roundID,
				TerminalStatus: roundStatusRunning,
			}
			rounds[roundID] = snapshot
		}
		snapshot.SessionKey = firstNonEmpty(snapshot.SessionKey, stringFromAny(row["session_key"]))
		snapshot.RoomID = firstNonEmpty(snapshot.RoomID, stringFromAny(row["room_id"]))
		snapshot.ConversationID = firstNonEmpty(snapshot.ConversationID, stringFromAny(row["conversation_id"]))
		snapshot.AgentID = firstNonEmpty(snapshot.AgentID, stringFromAny(row["agent_id"]))
		snapshot.SessionID = firstNonEmpty(snapshot.SessionID, stringFromAny(row["session_id"]))
		snapshot.ParentID = firstNonEmpty(snapshot.ParentID, stringFromAny(row["parent_id"]))
		if ts := messageTimestamp(row); ts > snapshot.LastTimestampMS {
			snapshot.LastTimestampMS = ts
		}
		if stringFromAny(row["role"]) == "result" {
			snapshot.HasResult = true
			snapshot.TerminalStatus = normalizeRoundStatusValue(row["subtype"])
			continue
		}
		if terminalStatus := assistantTerminalStatus(row); terminalStatus != roundStatusRunning {
			snapshot.TerminalStatus = terminalStatus
		}
	}

	result := make([]protocol.Message, 0, len(rows)+len(rounds))
	result = append(result, rows...)
	for roundID, snapshot := range rounds {
		if snapshot == nil || snapshot.HasResult {
			continue
		}
		if _, isActive := activeRoundIDs[roundID]; isActive {
			continue
		}
		if snapshot.TerminalStatus != roundStatusRunning {
			continue
		}
		timestamp := snapshot.LastTimestampMS + 1
		if timestamp <= 0 {
			timestamp = time.Now().UnixMilli()
		}
		payload := protocol.Message{
			"message_id":      "assistant_interrupt_" + roundID,
			"session_key":     snapshot.SessionKey,
			"room_id":         emptyStringToNil(snapshot.RoomID),
			"conversation_id": emptyStringToNil(snapshot.ConversationID),
			"agent_id":        snapshot.AgentID,
			"round_id":        roundID,
			"session_id":      emptyStringToNil(snapshot.SessionID),
			"role":            "assistant",
			"timestamp":       timestamp,
			"stop_reason":     "cancelled",
			"is_complete":     true,
			"content":         []map[string]any{},
			"result_summary": map[string]any{
				"message_id":      "result_" + roundID,
				"timestamp":       timestamp,
				"subtype":         "interrupted",
				"duration_ms":     0,
				"duration_api_ms": 0,
				"num_turns":       0,
				"is_error":        false,
			},
		}
		if strings.TrimSpace(snapshot.ParentID) != "" {
			payload["parent_id"] = snapshot.ParentID
		}
		result = append(result, payload)
	}

	sortHistoryRows(result)
	return result
}

func normalizeActiveRoundIDs(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeRoundStatusValue(value any) roundTerminalStatus {
	normalized := strings.ToLower(stringFromAny(value))
	switch normalized {
	case "", "running":
		return roundStatusRunning
	case "interrupted", "cancelled":
		return roundStatusInterrupted
	case "error":
		return roundStatusError
	default:
		return roundStatusSuccess
	}
}

func assistantTerminalStatus(row protocol.Message) roundTerminalStatus {
	if stringFromAny(row["role"]) != "assistant" {
		return roundStatusRunning
	}
	stopReason := strings.ToLower(stringFromAny(row["stop_reason"]))
	if stopReason == "" {
		return roundStatusRunning
	}
	switch stopReason {
	case "cancelled", "interrupted":
		return roundStatusInterrupted
	case "error":
		return roundStatusError
	default:
		return roundStatusSuccess
	}
}

func emptyStringToNil(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
