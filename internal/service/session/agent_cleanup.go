// INPUT: Agent 删除前快照出的全部普通与 Room 成员 Session。
// OUTPUT: 已关闭 runtime、已清次级引用且不留 transcript/meta 的 Agent Session 集合。
// POS: Agent 删除跨 Session 域的批量清理入口。
package session

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// DeleteAgentSessionArtifacts 清理 Agent 删除前固化的全部 Session 文件产物。
func (s *Service) DeleteAgentSessionArtifacts(
	ctx context.Context,
	agentValue protocol.Agent,
	sessions []protocol.Session,
) error {
	workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
	ownerUserID := strings.TrimSpace(agentValue.OwnerUserID)
	if ownerUserID == "" {
		ownerUserID = authctx.OwnerUserID(ctx)
	}
	sessionKeys := make([]string, 0, len(sessions))
	seenKeys := make(map[string]struct{}, len(sessions))
	for _, item := range sessions {
		sessionKey := strings.TrimSpace(item.SessionKey)
		if sessionKey == "" {
			continue
		}
		if _, exists := seenKeys[sessionKey]; exists {
			continue
		}
		seenKeys[sessionKey] = struct{}{}
		sessionKeys = append(sessionKeys, sessionKey)
		if err := s.closeSessionRuntimeForDeletion(sessionKey); err != nil {
			return err
		}
	}
	if s.deletion != nil {
		if err := s.deletion.CleanupSessionReferences(ctx, ownerUserID, sessionKeys); err != nil {
			return err
		}
	}
	history := s.history.ForOwner(ownerUserID)
	files := s.files.ForOwner(ownerUserID)
	for _, item := range sessions {
		for _, transcriptSessionID := range protocol.SessionTranscriptIDs(item) {
			if _, err := history.DeleteTranscriptSession(workspacePath, transcriptSessionID); err != nil {
				return err
			}
		}
		if _, err := files.DeleteSession(workspacePath, item.SessionKey); err != nil {
			return err
		}
	}
	return nil
}
