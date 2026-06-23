package workspace

import (
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func normalizeHistoryRows(rows []protocol.Message, activeRoundIDs map[string]struct{}) []protocol.Message {
	visibleRows, deliveryReceipts := splitExternalDeliveryReceipts(filterInternalHistoryRows(rows))
	compacted := compactMessages(visibleRows)
	normalized := normalizeCompactedHistoryRows(compacted, activeRoundIDs)
	return mergeExternalDeliveryReceipts(normalized, deliveryReceipts)
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
