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
	updated, roomID, err := s.fillGoalPreview(ctx, parsed, sessionKey, nextTitle)
	if err != nil || !updated {
		return err
	}
	s.broadcastResync(ctx, Request{
		SessionKey:         sessionKey,
		ConversationID:     parsed.ConversationID,
		ConversationRoomID: roomID,
	})
	return nil
}

func (s *Service) fillGoalPreview(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionKey string,
	title string,
) (bool, string, error) {
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return s.fillRoomGoalPreview(ctx, parsed.ConversationID, title)
	}
	updated, err := s.fillSessionGoalPreview(ctx, sessionKey, title)
	return updated, "", err
}

func (s *Service) fillRoomGoalPreview(ctx context.Context, conversationID string, title string) (bool, string, error) {
	if s.rooms == nil || strings.TrimSpace(conversationID) == "" {
		return false, "", nil
	}
	current, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil || current == nil {
		return false, "", err
	}
	if !isDefaultConversationTitle(current.Conversation.Title, current.Room.Name) {
		return false, "", nil
	}
	_, err = s.rooms.UpdateConversationTitle(ctx, current.Room.ID, current.Conversation.ID, title)
	return err == nil, current.Room.ID, err
}

func (s *Service) fillSessionGoalPreview(ctx context.Context, sessionKey string, title string) (bool, error) {
	if s.sessions == nil {
		return false, nil
	}
	current, err := s.sessions.GetSession(ctx, sessionKey)
	if err != nil || current == nil {
		return false, err
	}
	if !isDefaultSessionTitle(current.Title) {
		return false, nil
	}
	_, err = s.sessions.UpdateSessionTitle(ctx, sessionKey, title)
	return err == nil, err
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
