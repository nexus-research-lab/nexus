package workspace

import (
	"strings"
	"time"
)

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

func (m *liveManager) FlushActiveWrites(agentID string) {
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedAgentID == "" {
		return
	}
	m.flushWrites(normalizedAgentID, true)
}

func (m *liveManager) flushSettledWrites(agentID string) {
	m.flushWrites(agentID, false)
}

func (m *liveManager) flushWrites(agentID string, force bool) {
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
		if !force && now.Sub(writeState.LastChangeAt) < liveQuietWindow {
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
