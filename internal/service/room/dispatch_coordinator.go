package room

import (
	"strings"
	"sync"
)

// roomDispatchRegistry 为每个 Room conversation 提供独立的派发闸门。
//
// 闸门只负责保持 queue、wake、continuation 和 round finish 的顺序；
// runtime 执行本身不应依赖这把锁。引用计数保证空闲 conversation 不会
// 永久占用锁对象，也避免并发创建同一 conversation 的两把锁。
type roomDispatchRegistry struct {
	mu      sync.Mutex
	entries map[string]*roomDispatchEntry
}

type roomDispatchEntry struct {
	mu   sync.Mutex
	refs int
}

type roomDispatchLease struct {
	registry *roomDispatchRegistry
	key      string
	entry    *roomDispatchEntry
	once     sync.Once
}

func (r *roomDispatchRegistry) acquire(key string) *roomDispatchLease {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "room:unknown"
	}

	r.mu.Lock()
	if r.entries == nil {
		r.entries = make(map[string]*roomDispatchEntry)
	}
	entry := r.entries[key]
	if entry == nil {
		entry = &roomDispatchEntry{}
		r.entries[key] = entry
	}
	entry.refs++
	r.mu.Unlock()

	entry.mu.Lock()
	return &roomDispatchLease{
		registry: r,
		key:      key,
		entry:    entry,
	}
}

func (l *roomDispatchLease) Unlock() {
	if l == nil || l.registry == nil || l.entry == nil {
		return
	}
	l.once.Do(func() {
		l.entry.mu.Unlock()
		l.registry.mu.Lock()
		l.entry.refs--
		if l.entry.refs == 0 && l.registry.entries[l.key] == l.entry {
			delete(l.registry.entries, l.key)
		}
		l.registry.mu.Unlock()
	})
}

func roomDispatchKey(sessionKey string, conversationID string) string {
	sessionKey = strings.TrimSpace(sessionKey)
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = roomConversationIDFromSessionKey(sessionKey)
	}
	if conversationID != "" {
		// conversation ID 是 Room 的并发边界；shared session 与 agent session
		// 可能不同，但它们仍必须落在同一把派发闸门下。
		return "conversation:" + conversationID
	}
	if sessionKey != "" {
		return "session:" + sessionKey
	}
	return ""
}

func (s *RealtimeService) lockRoomDispatch(sessionKey string, conversationID string) *roomDispatchLease {
	if s == nil {
		return nil
	}
	return s.dispatch.acquire(roomDispatchKey(sessionKey, conversationID))
}
