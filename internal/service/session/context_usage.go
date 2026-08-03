// INPUT: 已认证 owner 下的结构化 Agent 或 Room session_key。
// OUTPUT: 各 Agent Session meta.json 中最后一次有效的上下文占用快照。
// POS: WebSocket 重绑定恢复持久化 context usage 的只读服务边界。
package session

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetPersistedContextUsageSnapshots 读取 DM 单 Agent 或 Room 全体 Agent 的最近快照。
func (s *Service) GetPersistedContextUsageSnapshots(
	ctx context.Context,
	rawSessionKey string,
) (map[string]protocol.ContextUsageData, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindAgent {
		usage, usageErr := s.getPersistedAgentContextUsage(ctx, sessionKey, parsed.AgentID)
		if usageErr != nil || usage == nil {
			return map[string]protocol.ContextUsageData{}, usageErr
		}
		return map[string]protocol.ContextUsageData{parsed.AgentID: *usage}, nil
	}
	if parsed.Kind != protocol.SessionKeyKindRoom {
		return nil, ErrSessionMutationUnsupported
	}

	roomSessions, err := s.repository.ListRoomSessions(
		ctx,
		authctx.OwnerUserID(ctx),
	)
	if err != nil {
		return nil, err
	}
	result := make(map[string]protocol.ContextUsageData)
	for _, item := range roomSessions {
		if item.ConversationID == nil || *item.ConversationID != parsed.ConversationID {
			continue
		}
		usage, usageErr := s.getPersistedAgentContextUsage(
			ctx,
			item.SessionKey,
			item.AgentID,
		)
		if usageErr != nil {
			return nil, usageErr
		}
		if usage != nil {
			result[item.AgentID] = *usage
		}
	}
	return result, nil
}

func (s *Service) getPersistedAgentContextUsage(
	ctx context.Context,
	sessionKey string,
	agentID string,
) (*protocol.ContextUsageData, error) {
	workspacePaths, err := s.resolveWorkspacePaths(ctx, agentID)
	if err != nil {
		return nil, err
	}
	item, _, err := s.ownerFiles(ctx).FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil || item.ContextUsage == nil || item.ContextUsage.MaxTokens <= 0 {
		return nil, nil
	}
	usage := *item.ContextUsage
	return &usage, nil
}
