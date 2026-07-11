package workspace

import (
	"context"
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
