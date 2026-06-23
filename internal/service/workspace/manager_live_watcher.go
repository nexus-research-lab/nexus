package workspace

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (m *liveManager) startWatcherLocked(agentID string, workspacePath string) (*agentWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	root := filepath.Clean(strings.TrimSpace(workspacePath))
	if err = os.MkdirAll(root, 0o755); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	state := &agentWatcher{
		AgentID:      agentID,
		Root:         root,
		Watcher:      watcher,
		Snapshots:    make(map[string]*string),
		Versions:     make(map[string]int),
		ActiveWrites: make(map[string]*activeWriteState),
		IgnoredUntil: make(map[string]time.Time),
	}
	if err = m.addWatchersLocked(state, root); err != nil {
		_ = watcher.Close()
		return nil, err
	}
	if err = m.captureSnapshotsLocked(state); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.Cancel = cancel
	m.watchers[agentID] = state
	go m.runWatcher(ctx, agentID)
	return state, nil
}

func (m *liveManager) addWatchersLocked(state *agentWatcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info == nil || !info.IsDir() {
			return nil
		}
		relativePath := "."
		if path != root {
			nextRelative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relativePath = filepath.ToSlash(nextRelative)
		}
		if relativePath != "." && shouldHideWorkspaceEntry(relativePath) {
			return filepath.SkipDir
		}
		return state.Watcher.Add(path)
	})
}

func (m *liveManager) captureSnapshotsLocked(state *agentWatcher) error {
	return filepath.Walk(state.Root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info == nil {
			return nil
		}
		relativePath, err := filepath.Rel(state.Root, path)
		if err != nil {
			return err
		}
		normalizedPath := normalizeLivePath(relativePath)
		if info.IsDir() {
			if normalizedPath != "" && normalizedPath != "." && shouldHideWorkspaceEntry(normalizedPath) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldHideWorkspaceEntry(normalizedPath) {
			return nil
		}
		snapshot := readWorkspaceSnapshot(path, info.Size())
		state.Snapshots[normalizedPath] = snapshot
		if snapshot != nil {
			state.Versions[normalizedPath] = 1
		}
		return nil
	})
}

func (m *liveManager) runWatcher(ctx context.Context, agentID string) {
	ticker := time.NewTicker(liveTickerInterval)
	defer ticker.Stop()

	for {
		m.mu.Lock()
		state := m.watchers[agentID]
		m.mu.Unlock()
		if state == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case event, ok := <-state.Watcher.Events:
			if !ok {
				return
			}
			m.handleFSEvent(agentID, event)
		case watchErr, ok := <-state.Watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("workspace watcher 错误", "agent_id", agentID, "err", watchErr)
		case <-ticker.C:
			m.flushSettledWrites(agentID)
		}
	}
}

func (m *liveManager) handleFSEvent(agentID string, event fsnotify.Event) {
	m.mu.Lock()
	state := m.watchers[agentID]
	if state == nil {
		m.mu.Unlock()
		return
	}

	info, statErr := os.Stat(event.Name)
	relativePath, ok := relativeLivePath(state.Root, event.Name)
	if !ok {
		m.mu.Unlock()
		return
	}
	if shouldHideWorkspaceEntry(relativePath) {
		m.mu.Unlock()
		return
	}
	if ignoreUntil, exists := state.IgnoredUntil[relativePath]; exists {
		if time.Now().UTC().Before(ignoreUntil) {
			m.mu.Unlock()
			return
		}
		delete(state.IgnoredUntil, relativePath)
	}

	if statErr == nil && info != nil && info.IsDir() && event.Has(fsnotify.Create) {
		_ = m.addWatchersLocked(state, event.Name)
		m.mu.Unlock()
		return
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) || os.IsNotExist(statErr) {
		version := state.Versions[relativePath] + 1
		state.Versions[relativePath] = version
		delete(state.Snapshots, relativePath)
		delete(state.ActiveWrites, relativePath)
		listeners := m.snapshotListenersLocked(agentID)
		m.mu.Unlock()
		m.dispatchListeners(listeners, LiveEvent{
			Type:      LiveEventFileDeleted,
			AgentID:   agentID,
			Path:      relativePath,
			Version:   version,
			Source:    LiveSourceAgent,
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		})
		return
	}

	if statErr != nil || info == nil || info.IsDir() {
		m.mu.Unlock()
		return
	}

	content := readWorkspaceSnapshot(event.Name, info.Size())
	writeState := state.ActiveWrites[relativePath]
	listeners := m.snapshotListenersLocked(agentID)
	now := time.Now().UTC()
	if writeState == nil {
		version := state.Versions[relativePath] + 1
		state.Versions[relativePath] = version
		writeState = &activeWriteState{
			BeforeContent: cloneStringPointer(state.Snapshots[relativePath]),
			Current:       cloneStringPointer(content),
			LastChangeAt:  now,
			Version:       version,
		}
		state.ActiveWrites[relativePath] = writeState
		m.mu.Unlock()
		m.dispatchListeners(listeners, LiveEvent{
			Type:      LiveEventFileWriteStart,
			AgentID:   agentID,
			Path:      relativePath,
			Version:   version,
			Source:    LiveSourceAgent,
			Timestamp: now.Format(time.RFC3339Nano),
		})
		m.dispatchListeners(listeners, LiveEvent{
			Type:            LiveEventFileWriteDelta,
			AgentID:         agentID,
			Path:            relativePath,
			Version:         version,
			Source:          LiveSourceAgent,
			ContentSnapshot: cloneStringPointer(content),
			Timestamp:       now.Format(time.RFC3339Nano),
		})
		return
	}

	writeState.LastChangeAt = now
	writeState.Current = cloneStringPointer(content)
	version := writeState.Version
	m.mu.Unlock()
	m.dispatchListeners(listeners, LiveEvent{
		Type:            LiveEventFileWriteDelta,
		AgentID:         agentID,
		Path:            relativePath,
		Version:         version,
		Source:          LiveSourceAgent,
		ContentSnapshot: cloneStringPointer(content),
		Timestamp:       now.Format(time.RFC3339Nano),
	})
}
