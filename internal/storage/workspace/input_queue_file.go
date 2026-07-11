package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *InputQueueStore) removeLocked(
	location InputQueueLocation,
	itemID string,
	action string,
) ([]protocol.InputQueueItem, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return s.snapshotLocked(location)
	}
	if err := s.appendActionLocked(location, map[string]any{
		"action":    action,
		"item_id":   itemID,
		"timestamp": time.Now().UnixMilli(),
	}); err != nil {
		return nil, err
	}
	return s.snapshotLocked(location)
}

func (s *InputQueueStore) snapshotLocked(location InputQueueLocation) ([]protocol.InputQueueItem, error) {
	path, err := s.pathForLocation(location)
	if err != nil {
		return nil, err
	}
	rows, err := s.files.readJSONL(path)
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.InputQueueItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	return replayInputQueueRows(location, rows), nil
}

func (s *InputQueueStore) appendActionLocked(location InputQueueLocation, row map[string]any) error {
	path, err := s.pathForLocation(location)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return s.files.appendJSONL(path, row)
}

func (s *InputQueueStore) pathForLocation(location InputQueueLocation) (string, error) {
	workspacePath := strings.TrimSpace(location.WorkspacePath)
	sessionKey := strings.TrimSpace(location.SessionKey)
	if workspacePath == "" {
		return "", errors.New("workspace_path is required")
	}
	if sessionKey == "" {
		return "", errors.New("session_key is required")
	}
	return s.paths.SessionInputQueuePath(workspacePath, sessionKey), nil
}
