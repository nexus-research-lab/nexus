package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	liveQuietWindow      = 1500 * time.Millisecond
	liveIgnoreWindow     = 2 * time.Second
	liveTickerInterval   = 400 * time.Millisecond
	liveMaxSnapshotBytes = 128 * 1024
)

type liveSubscription struct {
	AgentID  string
	Listener LiveListener
}

type activeWriteState struct {
	BeforeContent *string
	Current       *string
	LastChangeAt  time.Time
	Version       int
}

type agentWatcher struct {
	AgentID      string
	Root         string
	Watcher      *fsnotify.Watcher
	Cancel       context.CancelFunc
	RefCount     int
	Snapshots    map[string]*string
	Versions     map[string]int
	ActiveWrites map[string]*activeWriteState
	IgnoredUntil map[string]time.Time
}

type liveManager struct {
	mu            sync.Mutex
	subscriptions map[string]liveSubscription
	listeners     map[string]map[string]LiveListener
	watchers      map[string]*agentWatcher
}

func newLiveManager() *liveManager {
	return &liveManager{
		subscriptions: make(map[string]liveSubscription),
		listeners:     make(map[string]map[string]LiveListener),
		watchers:      make(map[string]*agentWatcher),
	}
}

func (m *liveManager) Subscribe(agentID string, workspacePath string, listener LiveListener) (string, error) {
	if listener == nil {
		return "", nil
	}
	normalizedAgentID := strings.TrimSpace(agentID)
	root := filepath.Clean(strings.TrimSpace(workspacePath))
	if normalizedAgentID == "" || root == "" {
		return "", nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	watcherState := m.watchers[normalizedAgentID]
	if watcherState == nil {
		created, err := m.startWatcherLocked(normalizedAgentID, root)
		if err != nil {
			return "", err
		}
		watcherState = created
	}
	watcherState.RefCount++

	token := newLiveToken()
	if m.listeners[normalizedAgentID] == nil {
		m.listeners[normalizedAgentID] = make(map[string]LiveListener)
	}
	m.listeners[normalizedAgentID][token] = listener
	m.subscriptions[token] = liveSubscription{
		AgentID:  normalizedAgentID,
		Listener: listener,
	}
	return token, nil
}

func (m *liveManager) Unsubscribe(token string) {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	subscription, ok := m.subscriptions[normalizedToken]
	if !ok {
		return
	}
	delete(m.subscriptions, normalizedToken)

	listeners := m.listeners[subscription.AgentID]
	if listeners != nil {
		delete(listeners, normalizedToken)
		if len(listeners) == 0 {
			delete(m.listeners, subscription.AgentID)
		}
	}

	watcherState := m.watchers[subscription.AgentID]
	if watcherState == nil {
		return
	}
	watcherState.RefCount--
	if watcherState.RefCount > 0 {
		return
	}

	if watcherState.Cancel != nil {
		watcherState.Cancel()
	}
	if watcherState.Watcher != nil {
		_ = watcherState.Watcher.Close()
	}
	delete(m.watchers, subscription.AgentID)
}

func (m *liveManager) SuppressWatcher(agentID string, relativePath string) {
	normalizedAgentID := strings.TrimSpace(agentID)
	normalizedPath := normalizeLivePath(relativePath)
	if normalizedAgentID == "" || normalizedPath == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	watcherState := m.watchers[normalizedAgentID]
	if watcherState == nil {
		return
	}
	watcherState.IgnoredUntil[normalizedPath] = time.Now().UTC().Add(liveIgnoreWindow)
}

func (m *liveManager) EmitAPIWrite(agentID string, relativePath string, content string) {
	normalizedAgentID := strings.TrimSpace(agentID)
	normalizedPath := normalizeLivePath(relativePath)
	if normalizedAgentID == "" || normalizedPath == "" {
		return
	}

	now := time.Now().UTC()
	var (
		before    *string
		version   int
		listeners []LiveListener
	)

	m.mu.Lock()
	listeners = m.snapshotListenersLocked(normalizedAgentID)
	if watcherState := m.watchers[normalizedAgentID]; watcherState != nil {
		before = cloneStringPointer(watcherState.Snapshots[normalizedPath])
		version = watcherState.Versions[normalizedPath] + 1
		watcherState.Versions[normalizedPath] = version
		watcherState.Snapshots[normalizedPath] = stringPointer(content)
		watcherState.IgnoredUntil[normalizedPath] = now.Add(liveIgnoreWindow)
		delete(watcherState.ActiveWrites, normalizedPath)
	} else {
		version = 1
	}
	m.mu.Unlock()

	if len(listeners) == 0 {
		return
	}

	contentPointer := stringPointer(content)
	baseEvent := LiveEvent{
		AgentID:   normalizedAgentID,
		Path:      normalizedPath,
		Version:   version,
		Source:    LiveSourceAPI,
		Timestamp: now.Format(time.RFC3339Nano),
	}
	m.dispatchListeners(listeners, cloneLiveEvent(baseEvent, func(event *LiveEvent) {
		event.Type = LiveEventFileWriteStart
	}))
	m.dispatchListeners(listeners, cloneLiveEvent(baseEvent, func(event *LiveEvent) {
		event.Type = LiveEventFileWriteDelta
		event.ContentSnapshot = cloneStringPointer(contentPointer)
	}))
	m.dispatchListeners(listeners, cloneLiveEvent(baseEvent, func(event *LiveEvent) {
		event.Type = LiveEventFileWriteEnd
		event.ContentSnapshot = cloneStringPointer(contentPointer)
		event.DiffStats = buildDiffStats(before, contentPointer)
	}))
}

func (m *liveManager) EmitAPIDelete(agentID string, relativePath string) {
	normalizedAgentID := strings.TrimSpace(agentID)
	normalizedPath := normalizeLivePath(relativePath)
	if normalizedAgentID == "" || normalizedPath == "" {
		return
	}

	now := time.Now().UTC()
	var (
		version   int
		listeners []LiveListener
	)

	m.mu.Lock()
	listeners = m.snapshotListenersLocked(normalizedAgentID)
	if watcherState := m.watchers[normalizedAgentID]; watcherState != nil {
		version = watcherState.Versions[normalizedPath] + 1
		watcherState.Versions[normalizedPath] = version
		watcherState.IgnoredUntil[normalizedPath] = now.Add(liveIgnoreWindow)
		delete(watcherState.ActiveWrites, normalizedPath)
		delete(watcherState.Snapshots, normalizedPath)
	} else {
		version = 1
	}
	m.mu.Unlock()

	if len(listeners) == 0 {
		return
	}

	m.dispatchListeners(listeners, LiveEvent{
		Type:      LiveEventFileDeleted,
		AgentID:   normalizedAgentID,
		Path:      normalizedPath,
		Version:   version,
		Source:    LiveSourceAPI,
		Timestamp: now.Format(time.RFC3339Nano),
	})
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
		if info == nil || info.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(state.Root, path)
		if err != nil {
			return err
		}
		normalizedPath := normalizeLivePath(relativePath)
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
		case <-state.Watcher.Errors:
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

func (m *liveManager) flushSettledWrites(agentID string) {
	type settledEvent struct {
		Listeners []LiveListener
		Event     LiveEvent
	}

	now := time.Now().UTC()
	pending := make([]settledEvent, 0)

	m.mu.Lock()
	state := m.watchers[agentID]
	if state == nil {
		m.mu.Unlock()
		return
	}
	for path, ignoredUntil := range state.IgnoredUntil {
		if now.After(ignoredUntil) {
			delete(state.IgnoredUntil, path)
		}
	}
	listeners := m.snapshotListenersLocked(agentID)
	for path, writeState := range state.ActiveWrites {
		if now.Sub(writeState.LastChangeAt) < liveQuietWindow {
			continue
		}
		state.Snapshots[path] = cloneStringPointer(writeState.Current)
		delete(state.ActiveWrites, path)

		pending = append(pending, settledEvent{
			Listeners: listeners,
			Event: LiveEvent{
				Type:            LiveEventFileWriteEnd,
				AgentID:         agentID,
				Path:            path,
				Version:         writeState.Version,
				Source:          LiveSourceAgent,
				ContentSnapshot: cloneStringPointer(writeState.Current),
				DiffStats:       buildDiffStats(writeState.BeforeContent, writeState.Current),
				Timestamp:       now.Format(time.RFC3339Nano),
			},
		})
	}
	m.mu.Unlock()

	for _, item := range pending {
		m.dispatchListeners(item.Listeners, item.Event)
	}
}

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

func buildDiffStats(before *string, after *string) *DiffStats {
	if before == nil && after == nil {
		return nil
	}
	beforeLines := splitLiveLines(before)
	afterLines := splitLiveLines(after)
	if len(beforeLines) == 0 && len(afterLines) == 0 {
		return nil
	}

	commonPrefix := 0
	for commonPrefix < len(beforeLines) && commonPrefix < len(afterLines) && beforeLines[commonPrefix] == afterLines[commonPrefix] {
		commonPrefix++
	}

	commonSuffix := 0
	for commonSuffix < len(beforeLines)-commonPrefix && commonSuffix < len(afterLines)-commonPrefix &&
		beforeLines[len(beforeLines)-1-commonSuffix] == afterLines[len(afterLines)-1-commonSuffix] {
		commonSuffix++
	}

	deletions := len(beforeLines) - commonPrefix - commonSuffix
	additions := len(afterLines) - commonPrefix - commonSuffix
	if additions < 0 {
		additions = 0
	}
	if deletions < 0 {
		deletions = 0
	}
	if additions == 0 && deletions == 0 && before != nil && after != nil && *before != *after {
		additions = len(afterLines)
		deletions = len(beforeLines)
	}
	if additions == 0 && deletions == 0 {
		return nil
	}
	return &DiffStats{
		Additions:    additions,
		Deletions:    deletions,
		ChangedLines: additions + deletions,
	}
}

func splitLiveLines(content *string) []string {
	if content == nil || *content == "" {
		return nil
	}
	return strings.Split(*content, "\n")
}

func newLiveToken() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buffer)
}
