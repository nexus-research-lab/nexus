package workspace

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	defaultMessageHistoryRoundPageSize = 3
	maxMessageHistoryRoundPageSize     = 10
	defaultMessageHistoryAroundLimit   = 2
	maxMessageHistoryAroundLimit       = 3
)

type historyPageGroup struct {
	CursorRoundID        string
	CursorRoundTimestamp int64
	Items                []protocol.Message
}

func normalizeRoundPageLimit(limit int) int {
	if limit <= 0 {
		return defaultMessageHistoryRoundPageSize
	}
	return min(limit, maxMessageHistoryRoundPageSize)
}

func normalizeRoundAroundLimit(limit int) int {
	if limit <= 0 {
		return defaultMessageHistoryAroundLimit
	}
	return min(limit, maxMessageHistoryAroundLimit)
}

func normalizeHistoryPageRow(row protocol.Message, collapseRoomAgentRounds bool) protocol.Message {
	if !collapseRoomAgentRounds {
		return row
	}
	roundID := stringFromAny(row["round_id"])
	if roundID == "" {
		return row
	}
	normalizedRoundID := normalizeRoomHistoryRoundID(roundID, stringFromAny(row["agent_id"]))
	if normalizedRoundID == "" || normalizedRoundID == roundID {
		return row
	}
	normalized := protocol.Clone(row)
	normalized["round_id"] = normalizedRoundID
	return normalized
}

func historyPageCursorRoundID(row protocol.Message, collapseRoomAgentRounds bool) string {
	roundID := stringFromAny(row["round_id"])
	if roundID != "" {
		if collapseRoomAgentRounds {
			return normalizeRoomHistoryRoundID(roundID, stringFromAny(row["agent_id"]))
		}
		return roundID
	}
	return stringFromAny(row["message_id"])
}

func historyPageGroupKey(row protocol.Message, collapseRoomAgentRounds bool) string {
	roundID := stringFromAny(row["round_id"])
	if roundID != "" {
		if collapseRoomAgentRounds {
			return "round:" + normalizeRoomHistoryRoundID(roundID, stringFromAny(row["agent_id"]))
		}
		return "round:" + roundID
	}

	messageID := stringFromAny(row["message_id"])
	if messageID != "" {
		return "message:" + messageID
	}
	return ""
}

func normalizeRoomHistoryRoundID(roundID string, agentID string) string {
	trimmedRoundID := strings.TrimSpace(roundID)
	trimmedAgentID := strings.TrimSpace(agentID)
	if trimmedRoundID == "" {
		return trimmedRoundID
	}
	if trimmedAgentID != "" {
		suffix := ":" + trimmedAgentID
		if strings.HasSuffix(trimmedRoundID, suffix) {
			return strings.TrimSuffix(trimmedRoundID, suffix)
		}
	}
	if strings.HasPrefix(trimmedRoundID, "room_mention_") ||
		strings.HasPrefix(trimmedRoundID, "room_directed_message_") {
		if index := strings.LastIndex(trimmedRoundID, ":"); index > 0 {
			return trimmedRoundID[:index]
		}
	}
	return trimmedRoundID
}

func findHistoryPageEndGroupIndex(
	groups []historyPageGroup,
	beforeRoundID string,
	beforeRoundTimestamp int64,
) int {
	if beforeRoundTimestamp <= 0 && beforeRoundID == "" {
		return len(groups)
	}
	if beforeRoundTimestamp <= 0 && beforeRoundID != "" {
		for index, group := range groups {
			if group.CursorRoundID == beforeRoundID {
				return index
			}
		}
		return 0
	}

	for index, group := range groups {
		if compareHistoryPageGroupCursor(group, beforeRoundID, beforeRoundTimestamp) >= 0 {
			return index
		}
	}
	return len(groups)
}

func compareHistoryPageGroupCursor(
	group historyPageGroup,
	beforeRoundID string,
	beforeRoundTimestamp int64,
) int {
	if group.CursorRoundTimestamp < beforeRoundTimestamp {
		return -1
	}
	if group.CursorRoundTimestamp > beforeRoundTimestamp {
		return 1
	}
	if beforeRoundID == "" {
		return 1
	}
	return strings.Compare(group.CursorRoundID, beforeRoundID)
}
