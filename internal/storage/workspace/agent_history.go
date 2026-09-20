// INPUT: owner-scoped transcript/overlay 与可选 Room SQL 摘要仓储。
// OUTPUT: 隔离的历史读写门面及完成回复摘要投影。
// POS: Agent 历史依赖装配与 owner 视图。
package workspace

import (
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

type agentHistoryCache struct {
	mu      sync.RWMutex
	entries map[string]transcriptCacheEntry
}

// AgentHistoryStore 负责读取 transcript 历史，并与 Nexus overlay 合并。
type AgentHistoryStore struct {
	replyPreviews *roomrepo.SQLRepository
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
			entries: make(map[string]transcriptCacheEntry),
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
		replyPreviews: s.replyPreviews,
		cache:         s.cache,
		runtimeRepair: s.runtimeRepair,
	}
}

// SetReplyPreviewRepository 注入当前宿主数据库的独立摘要投影。
func (s *AgentHistoryStore) SetReplyPreviewRepository(repository *roomrepo.SQLRepository) {
	s.replyPreviews = repository
}
