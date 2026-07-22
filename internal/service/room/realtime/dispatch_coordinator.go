package realtime

import (
	"strings"
	"sync"
)

// roomDispatchLease 把 queue、wake、continuation 和 round finish 的顺序
// 绑定到 conversation state；runtime 执行本身不依赖这把锁。
type roomDispatchLease struct {
	registry *roomRoundRegistry
	key      string
	state    *roomConversationState
	once     sync.Once
}

func (r *roomRoundRegistry) acquireDispatch(key string) *roomDispatchLease {
	if r == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "__room_unknown_conversation__"
	}

	r.mu.Lock()
	if r.conversations == nil {
		r.conversations = make(map[string]*roomConversationState)
	}
	state := r.conversations[key]
	if state == nil {
		state = newRoomConversationState()
		r.conversations[key] = state
	}
	state.dispatchRefs++
	r.mu.Unlock()

	state.dispatchMu.Lock()
	return &roomDispatchLease{
		registry: r,
		key:      key,
		state:    state,
	}
}

func (l *roomDispatchLease) Unlock() {
	if l == nil || l.registry == nil || l.state == nil {
		return
	}
	l.once.Do(func() {
		l.state.dispatchMu.Unlock()
		l.registry.releaseDispatch(l.key, l.state)
	})
}

func (r *roomRoundRegistry) releaseDispatch(key string, state *roomConversationState) {
	if r == nil || state == nil {
		return
	}
	r.mu.Lock()
	current := r.conversations[key]
	if current != state {
		r.mu.Unlock()
		return
	}
	if state.dispatchRefs > 0 {
		state.dispatchRefs--
	}
	last := state.dispatchRefs == 0
	r.mu.Unlock()
	if last {
		r.prune(key, state)
	}
}

func roomDispatchStateKey(sessionKey string, conversationID string) string {
	sessionKey = strings.TrimSpace(sessionKey)
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = roomConversationIDFromSessionKey(sessionKey)
	}
	if conversationID != "" {
		// conversation ID 是 Room 的并发边界；shared session 与 agent session
		// 可能不同，但它们仍必须落在同一份 conversation state 下。
		return conversationID
	}
	if sessionKey != "" {
		// 没有可解析 conversation 的 session 仍需保持彼此隔离，不能
		// 把所有未知会话合并到同一把锁。
		return "__room_dispatch_session:" + sessionKey
	}
	return "__room_unknown_conversation__"
}

func (s *Service) lockRoomDispatch(sessionKey string, conversationID string) *roomDispatchLease {
	if s == nil {
		return nil
	}
	return s.rounds.acquireDispatch(roomDispatchStateKey(sessionKey, conversationID))
}
