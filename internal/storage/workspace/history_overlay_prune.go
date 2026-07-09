package workspace

import (
	"errors"
	"os"
	"strings"
)

// RemoveOverlayRounds 从 Nexus overlay 中物理移除指定 round 的本地历史行。
func (s *AgentHistoryStore) RemoveOverlayRounds(
	workspacePath string,
	sessionKey string,
	roundIDs []string,
) (int, error) {
	removeSet := buildRoundRemoveSet(roundIDs)
	if len(removeSet) == 0 {
		return 0, nil
	}

	overlayPath := s.paths.SessionOverlayPath(workspacePath, sessionKey)
	rows, err := s.files.readJSONL(overlayPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	removed := 0
	nextRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if shouldRemoveOverlayRow(row, removeSet) {
			removed++
			continue
		}
		nextRows = append(nextRows, row)
	}
	if removed == 0 {
		return 0, nil
	}
	if err = s.files.replaceJSONL(overlayPath, nextRows); err != nil {
		return 0, err
	}
	return removed, nil
}

func buildRoundRemoveSet(roundIDs []string) map[string]struct{} {
	result := make(map[string]struct{}, len(roundIDs))
	for _, roundID := range roundIDs {
		if trimmed := strings.TrimSpace(roundID); trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

func shouldRemoveOverlayRow(row map[string]any, removeSet map[string]struct{}) bool {
	if _, ok := removeSet[strings.TrimSpace(stringFromAny(row["round_id"]))]; ok {
		return true
	}
	if stringFromAny(row[overlayKindField]) != "history_rewrite" {
		return false
	}
	_, ok := removeSet[strings.TrimSpace(stringFromAny(row["target_round_id"]))]
	return ok
}
