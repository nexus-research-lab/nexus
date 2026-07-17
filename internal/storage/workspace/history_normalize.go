// INPUT: overlay、transcript 与 runtime 产生的混合历史行。
// OUTPUT: 去重后的 durable 用户补充、assistant 与系统事件时间线。
// POS: workspace 历史在分页和 round 投影前的统一规范化入口。
package workspace

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func normalizeHistoryRows(rows []protocol.Message, activeRoundIDs map[string]struct{}) []protocol.Message {
	visibleRows, deliveryReceipts := splitExternalDeliveryReceipts(filterInternalHistoryRows(suppressDuplicatedGuidanceRows(rows)))
	compacted := compactMessages(visibleRows)
	normalized := normalizeCompactedHistoryRows(compacted, activeRoundIDs)
	return mergeExternalDeliveryReceipts(normalized, deliveryReceipts)
}

// durable user 已表达同一条引导时，不再重复展示 transcript 的 guided_input 系统投影。
func suppressDuplicatedGuidanceRows(rows []protocol.Message) []protocol.Message {
	durableSources := make(map[string]struct{})
	for _, row := range rows {
		if stringFromAny(row["role"]) != "user" {
			continue
		}
		if sourceRoundID := strings.TrimSpace(stringFromAny(row["source_round_id"])); sourceRoundID != "" {
			durableSources[sourceRoundID] = struct{}{}
		}
	}
	if len(durableSources) == 0 {
		return rows
	}
	filtered := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		metadata, _ := row["metadata"].(map[string]any)
		if stringFromAny(row["role"]) == "system" &&
			stringFromAny(metadata["subtype"]) == message.SystemMessageSubtypeGuidedInput {
			if _, ok := durableSources[strings.TrimSpace(stringFromAny(metadata["source_round_id"]))]; ok {
				continue
			}
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func filterInternalHistoryRows(rows []protocol.Message) []protocol.Message {
	if len(rows) == 0 {
		return rows
	}
	filtered := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		if shouldSkipInternalHistoryRow(row) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func shouldSkipInternalHistoryRow(row protocol.Message) bool {
	role := stringFromAny(row["role"])
	switch role {
	case "system":
		metadata, _ := row["metadata"].(map[string]any)
		return stringFromAny(metadata["subtype"]) == "api_retry"
	case "user":
		content := stringFromAny(row["content"])
		return message.IsInternalTranscriptInterruptPrompt(content)
	default:
		return false
	}
}

func normalizeCompactedHistoryRows(
	compacted []protocol.Message,
	activeRoundIDs map[string]struct{},
) []protocol.Message {
	materialized := materializeUnfinishedRounds(compacted, activeRoundIDs)
	return mergeRoundResultSummaries(materialized)
}
