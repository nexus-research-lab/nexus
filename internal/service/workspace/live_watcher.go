package workspace

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
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

func (m *liveManager) startWatcherLocked(
	agentID string,
	workspacePath string,
	workspaceRoot *confinedfs.Root,
) (*agentWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		workspaceRoot.Close()
		return nil, err
	}
	root := filepath.Clean(strings.TrimSpace(workspacePath))

	state := &agentWatcher{
		AgentID:      agentID,
		Root:         root,
		RootFS:       workspaceRoot,
		Watcher:      watcher,
		Snapshots:    make(map[string]*string),
		Versions:     make(map[string]int),
		ActiveWrites: make(map[string]*activeWriteState),
		IgnoredUntil: make(map[string]time.Time),
	}
	if err = m.addWatchersLocked(state, root); err != nil {
		_ = watcher.Close()
		_ = workspaceRoot.Close()
		return nil, err
	}
	if err = m.captureSnapshotsLocked(state); err != nil {
		_ = watcher.Close()
		_ = workspaceRoot.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	state.Cancel = cancel
	m.watchers[agentID] = state
	go m.runWatcher(ctx, agentID)
	return state, nil
}

func (m *liveManager) addWatchersLocked(state *agentWatcher, root string) error {
	relativeRoot := "."
	if filepath.Clean(root) != filepath.Clean(state.Root) {
		var ok bool
		relativeRoot, ok = relativeLivePath(state.Root, root)
		if !ok {
			return confinedfs.ErrParentTraversal
		}
	}
	confinedRoot, err := state.RootFS.OpenRootNoSymlink(
		filepath.ToSlash(relativeRoot),
	)
	if err != nil {
		return err
	}
	defer confinedRoot.Close()
	return confinedRoot.Walk(".", func(relativePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return handleWorkspaceWalkError(relativePath, entry, walkErr)
		}
		if entry == nil {
			return nil
		}
		normalizedPath := normalizeLivePath(relativePath)
		if normalizedPath == "" {
			normalizedPath = "."
		}
		info, err := lstatWorkspacePath(confinedRoot, normalizedPath)
		if err != nil {
			return handleWorkspaceWalkError(normalizedPath, entry, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if normalizedPath != "." && shouldHideWorkspaceEntry(normalizedPath) {
			return filepath.SkipDir
		}
		watchPath := root
		if normalizedPath != "." {
			watchPath = filepath.Join(root, filepath.FromSlash(normalizedPath))
		}
		if err = addPinnedWorkspaceWatch(state, confinedRoot, normalizedPath, watchPath); err != nil {
			return handleWorkspaceWalkError(normalizedPath, entry, err)
		}
		return nil
	})
}

func (m *liveManager) captureSnapshotsLocked(state *agentWatcher) error {
	root, err := state.RootFS.OpenRootNoSymlink(".")
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Walk(".", func(relativePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return handleWorkspaceWalkError(relativePath, entry, walkErr)
		}
		if entry == nil {
			return nil
		}
		normalizedPath := normalizeLivePath(relativePath)
		if normalizedPath == "" || normalizedPath == "." {
			return nil
		}
		info, infoErr := lstatWorkspacePath(root, normalizedPath)
		if os.IsNotExist(infoErr) {
			return nil
		}
		if infoErr != nil {
			return handleWorkspaceWalkError(normalizedPath, entry, infoErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if shouldHideWorkspaceEntry(normalizedPath) {
				return fs.SkipDir
			}
			return nil
		}
		if shouldHideWorkspaceEntry(normalizedPath) {
			return nil
		}
		snapshot := readWorkspaceSnapshot(state.RootFS, normalizedPath, info.Size())
		state.Snapshots[normalizedPath] = snapshot
		if snapshot != nil {
			state.Versions[normalizedPath] = 1
		}
		return nil
	})
}

func (m *liveManager) runWatcher(ctx context.Context, agentID string) {
	var settleTimer *time.Timer
	var settle <-chan time.Time
	defer func() {
		if settleTimer != nil {
			settleTimer.Stop()
		}
	}()

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
			if !m.handleFSEvent(agentID, event) || settle != nil {
				continue
			}
			if settleTimer == nil {
				settleTimer = time.NewTimer(liveQuietWindow)
			} else {
				settleTimer.Reset(liveQuietWindow)
			}
			settle = settleTimer.C
		case watchErr, ok := <-state.Watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("workspace watcher 错误", "agent_id", agentID, "err", watchErr)
		case <-settle:
			settle = nil
			if delay := m.flushSettledWrites(agentID); delay > 0 {
				settleTimer.Reset(delay)
				settle = settleTimer.C
			}
		}
	}
}

func (m *liveManager) handleFSEvent(agentID string, event fsnotify.Event) bool {
	resolved, ok := m.resolveFSEvent(agentID, event)
	if !ok || resolved.kind == liveFSEventIgnored {
		return false
	}
	m.mu.Lock()
	state := resolved.state
	if m.watchers[agentID] != state || m.ignoreLiveEventLocked(state, resolved.relativePath) {
		m.mu.Unlock()
		return false
	}
	events := m.applyFSEventLocked(agentID, resolved)
	listeners := m.snapshotListenersLocked(agentID)
	m.mu.Unlock()
	for _, liveEvent := range events {
		m.dispatchListeners(listeners, liveEvent)
	}
	return resolved.kind == liveFSEventWritten
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
	confinedRoot, rootErr := state.RootFS.OpenRootNoSymlink(".")
	if rootErr != nil {
		return resolvedLiveFSEvent{}, false
	}
	defer confinedRoot.Close()
	info, err := lstatWorkspacePath(confinedRoot, relativePath)
	switch {
	case err == nil && info != nil && info.IsDir() && event.Has(fsnotify.Create):
		resolved.kind = liveFSEventDirectoryCreated
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) || os.IsNotExist(err):
		resolved.kind = liveFSEventDeleted
	case err != nil || info == nil || info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		resolved.kind = liveFSEventIgnored
	default:
		resolved.kind = liveFSEventWritten
		resolved.content = readWorkspaceSnapshot(state.RootFS, relativePath, info.Size())
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

func addPinnedWorkspaceWatch(
	state *agentWatcher,
	root *confinedfs.Root,
	relativePath string,
	watchPath string,
) error {
	expected, err := lstatWorkspacePath(root, relativePath)
	if err != nil {
		return err
	}
	observed, err := os.Lstat(watchPath)
	if err != nil {
		return err
	}
	if observed.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, observed) {
		return confinedfs.ErrChanged
	}
	if err = state.Watcher.Add(watchPath); err != nil {
		return err
	}
	observed, err = os.Lstat(watchPath)
	if err != nil ||
		observed.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(expected, observed) {
		_ = state.Watcher.Remove(watchPath)
		if err != nil {
			return err
		}
		return confinedfs.ErrChanged
	}
	return nil
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
