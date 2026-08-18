// INPUT: source Session 的 append-only round marker 与目标完成轮次。
// OUTPUT: 截止目标轮次的 marker 前缀，写入新 Session overlay。
// POS: transcript fork 后保留 Nexus round 身份与用户输入元数据的最小投影。
package workspace

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
)

// ForkRoundMarkers 将 source marker 前缀复制到新的 Session。
func (s *AgentHistoryStore) ForkRoundMarkers(
	workspacePath string,
	sourceSessionKey string,
	targetSessionKey string,
	targetRoundID string,
) error {
	targetRoundID = strings.TrimSpace(targetRoundID)
	if targetRoundID == "" {
		return errors.New("target round id is required")
	}
	rows, err := s.files.readJSONLAt(
		workspacePath,
		s.paths.SessionOverlayPath(workspacePath, sourceSessionKey),
	)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	markers := make([]map[string]any, 0)
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) != overlayKindRoundMarker {
			continue
		}
		markers = append(markers, maps.Clone(row))
		if stringFromAny(row["round_id"]) == targetRoundID {
			break
		}
	}
	if len(markers) == 0 || stringFromAny(markers[len(markers)-1]["round_id"]) != targetRoundID {
		return nil
	}

	targetPath := s.paths.SessionOverlayPath(workspacePath, targetSessionKey)
	if existing, readErr := s.files.readJSONLAt(workspacePath, targetPath); readErr == nil && len(existing) > 0 {
		return fmt.Errorf("target session overlay already exists: %s", targetSessionKey)
	} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	for _, marker := range markers {
		if err = s.files.appendJSONLAt(workspacePath, targetPath, marker); err != nil {
			return err
		}
	}
	return nil
}
