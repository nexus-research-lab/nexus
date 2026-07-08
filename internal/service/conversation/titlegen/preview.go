package titlegen

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// FillEmptyPreviewFromGoal 用 Goal objective 填充仍为空/默认值的会话预览。
// 这条路径不调用模型，语义对齐 Codex create_goal 的 set_thread_preview_if_empty。
func (s *Service) FillEmptyPreviewFromGoal(ctx context.Context, sessionKey string, title string) error {
	if s == nil {
		return nil
	}
	sessionKey = strings.TrimSpace(sessionKey)
	nextTitle := strings.TrimSpace(title)
	if sessionKey == "" || nextTitle == "" {
		return nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	updated := false
	resolvedRoomID := ""
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if s.rooms == nil || strings.TrimSpace(parsed.ConversationID) == "" {
			return nil
		}
		current, err := s.rooms.GetConversationContext(ctx, parsed.ConversationID)
		if err != nil {
			return err
		}
		if current != nil && isDefaultConversationTitle(current.Conversation.Title, current.Room.Name) {
			resolvedRoomID = current.Room.ID
			if _, err = s.rooms.UpdateConversationTitle(ctx, current.Room.ID, current.Conversation.ID, nextTitle); err != nil {
				return err
			}
			updated = true
		}
	default:
		if s.sessions == nil {
			return nil
		}
		current, err := s.sessions.GetSession(ctx, sessionKey)
		if err != nil {
			return err
		}
		if current != nil && isDefaultSessionTitle(current.Title) {
			if _, err = s.sessions.UpdateSessionTitle(ctx, sessionKey, nextTitle); err != nil {
				return err
			}
			updated = true
		}
	}
	if updated {
		s.broadcastResync(ctx, Request{
			SessionKey:           sessionKey,
			ConversationID:       parsed.ConversationID,
			ConversationRoomID:   resolvedRoomID,
			ConversationRoomName: "",
		})
	}
	return nil
}

// ScheduleGoalTitleFromGoal 复用首条消息标题生成器，为 Goal 启动的新会话补标题总结。
func (s *Service) ScheduleGoalTitleFromGoal(ctx context.Context, item protocol.Goal, ownerUserID string, fallbackTitle string) {
	if s == nil {
		return
	}
	sessionKey := strings.TrimSpace(item.SessionKey)
	objective := strings.TrimSpace(item.Objective)
	if sessionKey == "" || objective == "" {
		return
	}
	request := Request{
		OwnerUserID:   strings.TrimSpace(ownerUserID),
		SessionKey:    sessionKey,
		Content:       objective,
		FallbackTitle: strings.TrimSpace(fallbackTitle),
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if strings.TrimSpace(parsed.ConversationID) == "" {
			return
		}
		request.SessionMessageCount = -1
		request.ConversationID = parsed.ConversationID
		request.ConversationMessageCount = 0
	} else {
		request.SessionMessageCount = 0
		request.ConversationMessageCount = -1
	}
	s.Schedule(ctx, request)
}
