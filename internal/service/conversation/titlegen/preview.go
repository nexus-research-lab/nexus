// INPUT: Goal aggregate plus a structured DM or Room session key.
// OUTPUT: Immediate fallback preview and asynchronous concise title targets for Goal-only conversations.
// POS: Goal control title bridge; treats a durable Goal as first user intent without requiring a normal chat message.
package titlegen

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
	if updated {
		conversationID := strings.TrimSpace(parsed.ConversationID)
		if conversationID == "" {
			conversationID = goalDMConversationID(parsed)
		}
		s.broadcastResync(ctx, Request{
			SessionKey:         sessionKey,
			ConversationID:     conversationID,
			ConversationRoomID: roomID,
		})
	}
	return err
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
	sessionUpdated, err := s.fillSessionGoalPreview(ctx, sessionKey, title)
	sessionErr := err
	conversationID := goalDMConversationID(parsed)
	if conversationID == "" {
		return sessionUpdated, "", sessionErr
	}
	conversationUpdated, roomID, conversationErr := s.fillRoomGoalPreview(ctx, conversationID, title)
	return sessionUpdated || conversationUpdated, roomID, errors.Join(sessionErr, conversationErr)
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
	if sessionUsesConversationTitle(current) {
		return false, nil
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
		if conversationID := goalDMConversationID(parsed); conversationID != "" {
			request.ConversationID = conversationID
			request.ConversationMessageCount = 0
		} else {
			request.ConversationMessageCount = -1
		}
	}
	s.Schedule(ctx, request)
}

// RepairGoalTitleFromGoal replays the idempotent Goal title projection for a
// durable Goal recovered after restart or a lost control response.
func (s *Service) RepairGoalTitleFromGoal(
	ctx context.Context,
	item protocol.Goal,
	ownerUserID string,
) error {
	if s == nil {
		return nil
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil
	}
	ctx = authctx.WithPrincipal(context.WithoutCancel(ctx), &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	})
	fallbackTitle := goalFallbackTitle(item)
	err := s.FillEmptyPreviewFromGoal(ctx, item.SessionKey, fallbackTitle)
	s.ScheduleGoalTitleFromGoal(ctx, item, ownerUserID, fallbackTitle)
	return err
}

func goalFallbackTitle(item protocol.Goal) string {
	if title := protocol.GoalMetadataString(item.Metadata, protocol.GoalMetadataRoomGoalLoopTitle); title != "" {
		return title
	}
	return strings.TrimSpace(item.Objective)
}

func goalDMConversationID(parsed protocol.SessionKey) string {
	if parsed.Kind != protocol.SessionKeyKindAgent ||
		protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) != protocol.SessionChannelWebSocketSegment ||
		strings.TrimSpace(parsed.ChatType) != "dm" {
		return ""
	}
	return strings.TrimSpace(parsed.Ref)
}
