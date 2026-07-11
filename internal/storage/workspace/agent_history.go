package workspace

import (
	"sync"
)

// AgentHistoryStore 负责读取 transcript 历史，并与 Nexus overlay 合并。
type AgentHistoryStore struct {
	paths *Store
	files *SessionFileStore

	cacheMu      sync.RWMutex
	messageCache map[string]transcriptCacheEntry
}

// NewAgentHistoryStore 创建 DM 历史读写门面。
func NewAgentHistoryStore(root string) *AgentHistoryStore {
	return &AgentHistoryStore{
		paths:        New(root),
		files:        NewSessionFileStore(root),
		messageCache: make(map[string]transcriptCacheEntry),
	}
}
