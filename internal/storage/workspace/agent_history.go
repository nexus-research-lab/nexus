package workspace

import (
	"strings"
	"sync"
)

type agentHistoryCache struct {
	mu       sync.RWMutex
	messages map[string]transcriptCacheEntry
}

// AgentHistoryStore 负责读取 transcript 历史，并与 Nexus overlay 合并。
type AgentHistoryStore struct {
	paths         *Store
	files         *SessionFileStore
	readModel     *historyReadModel
	ownerUserID   string
	cache         *agentHistoryCache
	runtimeRepair *runtimePermissionRepair
}

// NewAgentHistoryStore 创建 DM 历史读写门面。
func NewAgentHistoryStore(root string) *AgentHistoryStore {
	return &AgentHistoryStore{
		paths:     New(root),
		files:     NewSessionFileStore(root),
		readModel: sharedHistoryReadModel(root),
		cache: &agentHistoryCache{
			messages: make(map[string]transcriptCacheEntry),
		},
		runtimeRepair: newRuntimePermissionRepair(),
	}
}

// ForOwner 返回绑定到单个 owner workspace/runtime 树的历史视图。
func (s *AgentHistoryStore) ForOwner(ownerUserID string) *AgentHistoryStore {
	if s == nil {
		return nil
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	return &AgentHistoryStore{
		paths:         s.paths,
		files:         s.files.ForOwner(ownerUserID),
		readModel:     s.readModel,
		ownerUserID:   ownerUserID,
		cache:         s.cache,
		runtimeRepair: s.runtimeRepair,
	}
}
