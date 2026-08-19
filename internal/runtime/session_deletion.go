// INPUT: 要从 Session 配置域删除的精确 runtime session_key。
// OUTPUT: 先于 CloseSession 生效的一次性 admission block，成功删除后保留进程期墓碑。
// POS: Session 持久删除和 runtime GetOrCreate/StartRound 之间的竞态栅栏。
package runtime

import (
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrRuntimeSessionDeleted 表示 session 已进入持久删除流程，runtime 不得重新创建。
var ErrRuntimeSessionDeleted = errors.New("runtime session is deleting or deleted")

// SessionDeletionLease 只能用于撤销创建它的临时删除栅栏。
type SessionDeletionLease struct {
	sessionKey string
	blockKey   string
	token      uint64
}

// BeginSessionDeletion 在任何 runtime 关闭或文件删除前阻断 exact session_key 的新 admission。
func (m *Manager) BeginSessionDeletion(sessionKey string) (SessionDeletionLease, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return SessionDeletionLease{}, errors.New("session_key is required")
	}
	blockKey := runtimeSessionDeletionBlockKey(sessionKey)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessionDeletionBlocks[blockKey]; exists {
		return SessionDeletionLease{}, ErrRuntimeSessionDeleted
	}
	m.nextSessionDeletionID++
	token := m.nextSessionDeletionID
	if token == 0 {
		m.nextSessionDeletionID++
		token = m.nextSessionDeletionID
	}
	m.sessionDeletionBlocks[blockKey] = token
	return SessionDeletionLease{sessionKey: sessionKey, blockKey: blockKey, token: token}, nil
}

// AbortSessionDeletion 只在持久删除尚未提交时解除调用方自己的 admission block。
// 成功删除不调用该方法，使同 key 在本进程内保持不可复活。
func (m *Manager) AbortSessionDeletion(lease SessionDeletionLease) {
	if strings.TrimSpace(lease.blockKey) == "" || lease.token == 0 {
		return
	}
	m.mu.Lock()
	if m.sessionDeletionBlocks[lease.blockKey] == lease.token {
		delete(m.sessionDeletionBlocks, lease.blockKey)
	}
	m.mu.Unlock()
}

func runtimeSessionDeletionBlockKey(sessionKey string) string {
	sessionKey = strings.TrimSpace(sessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	return strings.TrimSpace(parsed.AgentID) + "\x00" +
		protocol.LegacySessionDirectoryIdentity(sessionKey)
}
