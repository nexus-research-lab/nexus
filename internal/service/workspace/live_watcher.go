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

type liveFSEventKind uint8

const (
	liveFSEventIgnored liveFSEventKind = iota
	liveFSEventDirectoryCreated
	liveFSEventDeleted
	liveFSEventWritten
)

type resolvedLiveFSEvent struct {
	state        *agentWatcher
	name         string
	relativePath string
	kind         liveFSEventKind
	content      *string
}

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
	resolved, ok := m.resolveFSEvent(agentID, event)
	if !ok || resolved.kind == liveFSEventIgnored {
		return
	}
	m.mu.Lock()
	state := resolved.state
	if m.watchers[agentID] != state || m.ignoreLiveEventLocked(state, resolved.relativePath) {
		m.mu.Unlock()
		return
	}
	events := m.applyFSEventLocked(agentID, resolved)
	listeners := m.snapshotListenersLocked(agentID)
	m.mu.Unlock()
	for _, liveEvent := range events {
		m.dispatchListeners(listeners, liveEvent)
	}
}

func (m *liveManager) resolveFSEvent(agentID string, event fsnotify.Event) (resolvedLiveFSEvent, bool) {
	m.mu.Lock()
	state := m.watchers[agentID]
	m.mu.Unlock()
	if state == nil {
		return resolvedLiveFSEvent{}, false
	}
	relativePath, ok := relativeLivePath(state.Root, event.Name)
	if !ok || shouldHideWorkspaceEntry(relativePath) {
		return resolvedLiveFSEvent{}, false
	}
	resolved := resolvedLiveFSEvent{state: state, name: event.Name, relativePath: relativePath}
	info, err := os.Stat(event.Name)
	switch {
	case err == nil && info != nil && info.IsDir() && event.Has(fsnotify.Create):
		resolved.kind = liveFSEventDirectoryCreated
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) || os.IsNotExist(err):
		resolved.kind = liveFSEventDeleted
	case err != nil || info == nil || info.IsDir():
		resolved.kind = liveFSEventIgnored
	default:
		resolved.kind = liveFSEventWritten
		resolved.content = readWorkspaceSnapshot(event.Name, info.Size())
	}
	return resolved, true
}

func (m *liveManager) ignoreLiveEventLocked(state *agentWatcher, relativePath string) bool {
	ignoreUntil, exists := state.IgnoredUntil[relativePath]
	if !exists {
		return false
	}
	if time.Now().UTC().Before(ignoreUntil) {
		return true
	}
	delete(state.IgnoredUntil, relativePath)
	return false
}

func (m *liveManager) applyFSEventLocked(agentID string, event resolvedLiveFSEvent) []LiveEvent {
	switch event.kind {
	case liveFSEventDirectoryCreated:
		_ = m.addWatchersLocked(event.state, event.name)
		return nil
	case liveFSEventDeleted:
		return []LiveEvent{deleteLiveFileLocked(agentID, event.state, event.relativePath)}
	case liveFSEventWritten:
		return writeLiveFileLocked(agentID, event.state, event.relativePath, event.content)
	default:
		return nil
	}
}

func deleteLiveFileLocked(agentID string, state *agentWatcher, relativePath string) LiveEvent {
	version := state.Versions[relativePath] + 1
	state.Versions[relativePath] = version
	delete(state.Snapshots, relativePath)
	delete(state.ActiveWrites, relativePath)
	return LiveEvent{
		Type:      LiveEventFileDeleted,
		AgentID:   agentID,
		Path:      relativePath,
		Version:   version,
		Source:    LiveSourceAgent,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func writeLiveFileLocked(
	agentID string,
	state *agentWatcher,
	relativePath string,
	content *string,
) []LiveEvent {
	now := time.Now().UTC()
	writeState := state.ActiveWrites[relativePath]
	if writeState != nil {
		writeState.LastChangeAt = now
		writeState.Current = cloneStringPointer(content)
		return []LiveEvent{liveWriteDeltaEvent(agentID, relativePath, writeState.Version, content, now)}
	}
	version := state.Versions[relativePath] + 1
	state.Versions[relativePath] = version
	state.ActiveWrites[relativePath] = &activeWriteState{
		BeforeContent: cloneStringPointer(state.Snapshots[relativePath]),
		Current:       cloneStringPointer(content),
		LastChangeAt:  now,
		Version:       version,
	}
	return []LiveEvent{
		{
			Type:      LiveEventFileWriteStart,
			AgentID:   agentID,
			Path:      relativePath,
			Version:   version,
			Source:    LiveSourceAgent,
			Timestamp: now.Format(time.RFC3339Nano),
		},
		liveWriteDeltaEvent(agentID, relativePath, version, content, now),
	}
}

func liveWriteDeltaEvent(
	agentID string,
	relativePath string,
	version int,
	content *string,
	now time.Time,
) LiveEvent {
	return LiveEvent{
		Type:            LiveEventFileWriteDelta,
		AgentID:         agentID,
		Path:            relativePath,
		Version:         version,
		Source:          LiveSourceAgent,
		ContentSnapshot: cloneStringPointer(content),
		Timestamp:       now.Format(time.RFC3339Nano),
	}
}
