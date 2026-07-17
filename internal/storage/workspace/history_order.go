package workspace

import (
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func sortHistoryRows(rows []protocol.Message) {
	sort.SliceStable(rows, func(i int, j int) bool {
		return compareHistoryRowOrder(rows[i], rows[j]) < 0
	})
}

func compareHistoryRowOrder(left protocol.Message, right protocol.Message) int {
	leftTimestamp := messageTimestamp(left)
	rightTimestamp := messageTimestamp(right)
	if leftTimestamp != rightTimestamp {
		if leftTimestamp < rightTimestamp {
			return -1
		}
		return 1
	}

	leftRoundID := stringFromAny(left["round_id"])
	rightRoundID := stringFromAny(right["round_id"])
	if leftRoundID != "" && leftRoundID == rightRoundID {
		leftOrder := historyRoleOrder(left)
		rightOrder := historyRoleOrder(right)
		if leftOrder != rightOrder {
			if leftOrder < rightOrder {
				return -1
			}
			return 1
		}
	}
	leftDisplayOrder := protocol.Int64FromAny(left["display_order"])
	rightDisplayOrder := protocol.Int64FromAny(right["display_order"])
	if leftDisplayOrder != 0 && rightDisplayOrder != 0 && leftDisplayOrder != rightDisplayOrder {
		if leftDisplayOrder < rightDisplayOrder {
			return -1
		}
		return 1
	}

	leftMessageID := stringFromAny(left["message_id"])
	rightMessageID := stringFromAny(right["message_id"])
	if leftMessageID != rightMessageID {
		return strings.Compare(leftMessageID, rightMessageID)
	}
	return 0
}

func historyRoleOrder(row protocol.Message) int {
	switch stringFromAny(row["role"]) {
	case "user":
		return 0
	case "assistant", "system", "task_progress":
		return 1
	case "result":
		return 2
	default:
		return 3
	}
}
