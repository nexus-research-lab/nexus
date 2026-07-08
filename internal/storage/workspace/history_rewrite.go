package workspace

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func applyHistoryRewrites(rows []protocol.Message, rewrites []historyRewriteMarker) []protocol.Message {
	if len(rows) == 0 || len(rewrites) == 0 {
		return rows
	}
	current := rows
	for _, rewrite := range rewrites {
		current = applyHistoryRewrite(current, rewrite)
	}
	return current
}

func applyHistoryRewrite(rows []protocol.Message, rewrite historyRewriteMarker) []protocol.Message {
	targetRoundID := strings.TrimSpace(rewrite.TargetRoundID)
	if targetRoundID == "" {
		return rows
	}
	targetRow, ok := findHistoryRewriteTargetRow(rows, targetRoundID)
	if !ok {
		return rows
	}

	replacementRoundID := strings.TrimSpace(rewrite.ReplacementRoundID)
	filtered := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		if roundID == targetRoundID {
			continue
		}
		if replacementRoundID != "" && roundID == replacementRoundID {
			filtered = append(filtered, row)
			continue
		}
		if rewrite.Timestamp > 0 && messageTimestamp(row) >= rewrite.Timestamp {
			filtered = append(filtered, row)
			continue
		}
		if compareHistoryRowOrder(row, targetRow) < 0 {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func findHistoryRewriteTargetRow(rows []protocol.Message, targetRoundID string) (protocol.Message, bool) {
	sortedRows := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(stringFromAny(row["round_id"])) == targetRoundID {
			sortedRows = append(sortedRows, row)
		}
	}
	if len(sortedRows) == 0 {
		return nil, false
	}
	sortHistoryRows(sortedRows)
	for _, row := range sortedRows {
		if strings.TrimSpace(stringFromAny(row["role"])) == "user" {
			return row, true
		}
	}
	return sortedRows[0], true
}
