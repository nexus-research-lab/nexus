package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (m *liveManager) snapshotListenersLocked(agentID string) []LiveListener {
	entries := m.listeners[agentID]
	if len(entries) == 0 {
		return nil
	}
	result := make([]LiveListener, 0, len(entries))
	for _, listener := range entries {
		if listener != nil {
			result = append(result, listener)
		}
	}
	return result
}

func (m *liveManager) dispatchListeners(listeners []LiveListener, event LiveEvent) {
	if len(listeners) == 0 {
		return
	}
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		listener(cloneLiveEvent(event, nil))
	}
}

func normalizeLivePath(relativePath string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(relativePath))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	return normalized
}

func relativeLivePath(root string, absolutePath string) (string, bool) {
	relativePath, err := filepath.Rel(filepath.Clean(root), filepath.Clean(absolutePath))
	if err != nil {
		return "", false
	}
	normalized := normalizeLivePath(relativePath)
	if normalized == "" || normalized == "." {
		return "", false
	}
	return normalized, true
}

func readWorkspaceSnapshot(path string, size int64) *string {
	if size > liveMaxSnapshotBytes {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(content)
	return &text
}

func stringPointer(value string) *string {
	normalized := value
	return &normalized
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneLiveEvent(event LiveEvent, mutate func(*LiveEvent)) LiveEvent {
	cloned := event
	cloned.SessionKey = cloneStringPointer(event.SessionKey)
	cloned.ToolUseID = cloneStringPointer(event.ToolUseID)
	cloned.ContentSnapshot = cloneStringPointer(event.ContentSnapshot)
	cloned.AppendedText = cloneStringPointer(event.AppendedText)
	if event.DiffStats != nil {
		diff := *event.DiffStats
		cloned.DiffStats = &diff
	}
	if mutate != nil {
		mutate(&cloned)
	}
	return cloned
}

func newLiveToken() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buffer)
}
