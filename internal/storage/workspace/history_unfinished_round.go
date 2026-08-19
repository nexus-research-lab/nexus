// INPUT: compact 后的历史 rows 与当前 active physical round 集合。
// OUTPUT: 为已离开 active 且无终态的 round 稳定补齐 interrupted assistant。
// POS: canonical history normalize 的未完成轮次收口阶段。
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
		ControlOnly     bool
		TerminalStatus  roundTerminalStatus
	}

	rounds := make(map[string]*roundSnapshot)
	roundOrder := make([]string, 0)
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
			roundOrder = append(roundOrder, roundID)
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
		if terminalControlMessage(row) {
			snapshot.ControlOnly = true
			snapshot.TerminalStatus = roundStatusSuccess
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
	// 合成 interrupt 的同 timestamp 顺序必须继承 canonical row 首次出现顺序；
	// map 迭代会让 before cursor 与 seek index 在相同输入上随机漂移。
	for _, roundID := range roundOrder {
		snapshot := rounds[roundID]
		if snapshot == nil || snapshot.HasResult || snapshot.ControlOnly {
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

// terminalControlMessage 识别不进入 runtime 的宿主控制记录。goal_set metadata
// 兼容 control_only 字段上线前已经持久化的 Goal 控制历史。
func terminalControlMessage(row protocol.Message) bool {
	if boolValueAny(row["control_only"]) {
		return true
	}
	switch metadata := row["metadata"].(type) {
	case map[string]string:
		return strings.TrimSpace(metadata["subtype"]) == "goal_set"
	case map[string]any:
		return stringFromAny(metadata["subtype"]) == "goal_set"
	default:
		return false
	}
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
