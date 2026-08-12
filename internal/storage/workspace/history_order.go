package workspace

import (
	"sort"

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
	leftControl := terminalControlMessage(left)
	rightControl := terminalControlMessage(right)
	if leftControl != rightControl {
		// Goal continuation 只有在 host control 已经 durable 后才允许启动；
		// 两者落在同一毫秒时，这条因果关系比跨存储来源的合并顺序更可靠。
		if leftControl {
			return -1
		}
		return 1
	}

	// 不同 round 在同一毫秒且没有共同 display_order 时，保留各历史来源完成
	// 合并后的稳定顺序。随机 message_id 的字典序不能反向重排用户控制记录
	// 与随后启动的 Goal continuation。
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
