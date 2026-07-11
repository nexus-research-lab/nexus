package goal

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrGoalContinuationTargetMissing 表示 Goal 所属的 agent/room/conversation 已不存在。
var ErrGoalContinuationTargetMissing = errors.New("goal continuation target missing")

// DeleteGoalsForAgent 删除指定 Agent 关联的全部 Goal。
func (s *Service) DeleteGoalsForAgent(ctx context.Context, agentID string) (int, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return 0, nil
	}
	return s.deleteGoalsMatching(ctx, func(_ protocol.Goal, parsed protocol.SessionKey) bool {
		return parsed.Kind == protocol.SessionKeyKindAgent && parsed.AgentID == agentID
	})
}

// DeleteGoalsForRoomConversations 删除 Room conversation 关联的共享与成员侧 Goal。
func (s *Service) DeleteGoalsForRoomConversations(ctx context.Context, conversationIDs []string) (int, error) {
	conversationIDSet := normalizeConversationIDSet(conversationIDs)
	if len(conversationIDSet) == 0 {
		return 0, nil
	}
	return s.deleteGoalsMatching(ctx, func(_ protocol.Goal, parsed protocol.SessionKey) bool {
		if parsed.Kind == protocol.SessionKeyKindRoom {
			_, ok := conversationIDSet[parsed.ConversationID]
			return ok
		}
		if parsed.Kind == protocol.SessionKeyKindAgent {
			_, ok := conversationIDSet[parsed.Ref]
			return ok
		}
		return false
	})
}

// DeleteGoalsForRoomMember 删除指定 Room 成员在 conversation 中的成员侧 Goal。
func (s *Service) DeleteGoalsForRoomMember(ctx context.Context, agentID string, conversationIDs []string) (int, error) {
	agentID = strings.TrimSpace(agentID)
	conversationIDSet := normalizeConversationIDSet(conversationIDs)
	if agentID == "" || len(conversationIDSet) == 0 {
		return 0, nil
	}
	return s.deleteGoalsMatching(ctx, func(_ protocol.Goal, parsed protocol.SessionKey) bool {
		if parsed.Kind != protocol.SessionKeyKindAgent || parsed.AgentID != agentID {
			return false
		}
		_, ok := conversationIDSet[parsed.Ref]
		return ok
	})
}

func (s *Service) deleteGoalsMatching(
	ctx context.Context,
	matches func(protocol.Goal, protocol.SessionKey) bool,
) (int, error) {
	if matches == nil || s == nil || s.repo == nil || !s.config.GoalEnabled {
		return 0, nil
	}
	items, err := s.repo.ListGoals(ctx)
	if err != nil {
		return 0, err
	}
	deletedCount := 0
	for _, item := range items {
		parsed := protocol.ParseSessionKey(item.SessionKey)
		if !matches(item, parsed) {
			continue
		}
		deleted, deleteErr := s.deleteGoal(ctx, item, protocol.GoalUpdateSourceSystem)
		if deleteErr != nil {
			return deletedCount, deleteErr
		}
		if deleted {
			deletedCount++
		}
	}
	return deletedCount, nil
}

func normalizeConversationIDSet(conversationIDs []string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, conversationID := range conversationIDs {
		normalized := strings.TrimSpace(conversationID)
		if normalized == "" {
			continue
		}
		result[normalized] = struct{}{}
	}
	return result
}
