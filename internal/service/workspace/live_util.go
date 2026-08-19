package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
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

func readWorkspaceSnapshot(
	root *confinedfs.Root,
	relativePath string,
	size int64,
) *string {
	if size > liveMaxSnapshotBytes {
		return nil
	}
	relativePath = normalizeLivePath(relativePath)
	if root == nil || relativePath == "" || relativePath == "." {
		return nil
	}
	parent, err := root.OpenRootNoSymlink(path.Dir(relativePath))
	if err != nil {
		return nil
	}
	defer parent.Close()
	file, err := parent.OpenFileNoSymlink(path.Base(relativePath), os.O_RDONLY, 0)
	if err != nil {
		return nil
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, liveMaxSnapshotBytes+1))
	if err != nil {
		return nil
	}
	if len(content) > liveMaxSnapshotBytes {
		return nil
	}
	text := string(content)
	return &text
}

func lstatWorkspacePath(root *confinedfs.Root, relativePath string) (os.FileInfo, error) {
	relativePath = normalizeLivePath(relativePath)
	if relativePath == "" || relativePath == "." {
		return root.Lstat(".")
	}
	parent, err := root.OpenRootNoSymlink(path.Dir(relativePath))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return parent.Lstat(path.Base(relativePath))
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
