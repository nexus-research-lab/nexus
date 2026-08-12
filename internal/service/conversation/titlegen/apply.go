package titlegen

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) applySessionTitle(ctx context.Context, sessionKey string, title string, fallbackTitle string) (bool, error) {
	if s.sessions == nil {
		return false, nil
	}
	current, err := s.sessions.GetSession(ctx, sessionKey)
	if err != nil {
		return false, err
	}
	if current == nil || sessionUsesConversationTitle(current) || !canReplaceSessionTitle(current.Title, fallbackTitle) {
		return false, nil
	}
	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		return false, nil
	}
	_, err = s.sessions.UpdateSessionTitle(ctx, sessionKey, nextTitle)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) canAutoUpdateSession(ctx context.Context, sessionKey string, fallbackTitle string) (bool, error) {
	if s.sessions == nil {
		return false, nil
	}
	current, err := s.sessions.GetSession(ctx, sessionKey)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, nil
	}
	if sessionUsesConversationTitle(current) {
		return false, nil
	}
	return canReplaceSessionTitle(current.Title, fallbackTitle), nil
}

// sessionUsesConversationTitle 判断 Session 是否只是 Room conversation 的 Agent 投影。
// 这类 Session 的标题权威在 SQL conversation，workspace 只保存 runtime 进度。
func sessionUsesConversationTitle(current *protocol.Session) bool {
	if current == nil {
		return false
	}
	for _, value := range []*string{
		current.RoomSessionID,
		current.RoomID,
		current.ConversationID,
	} {
		if value != nil && strings.TrimSpace(*value) != "" {
			return true
		}
	}
	return false
}

func (s *Service) applyConversationTitle(
	ctx context.Context,
	conversationID string,
	roomID string,
	title string,
	fallbackTitle string,
) (bool, error) {
	if s.rooms == nil {
		return false, nil
	}
	current, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return false, err
	}
	if current == nil || !canReplaceConversationTitle(current.Conversation.Title, current.Room.Name, fallbackTitle) {
		return false, nil
	}
	resolvedRoomID := strings.TrimSpace(roomID)
	if resolvedRoomID == "" {
		resolvedRoomID = current.Room.ID
	}
	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		return false, nil
	}
	_, err = s.rooms.UpdateConversationTitle(ctx, resolvedRoomID, conversationID, nextTitle)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) canAutoUpdateConversation(
	ctx context.Context,
	conversationID string,
	roomID string,
	fallbackTitle string,
) (bool, string, error) {
	if s.rooms == nil {
		return false, "", nil
	}
	current, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return false, "", err
	}
	if current == nil {
		return false, "", nil
	}
	resolvedRoomID := strings.TrimSpace(roomID)
	if resolvedRoomID == "" {
		resolvedRoomID = current.Room.ID
	}
	return canReplaceConversationTitle(current.Conversation.Title, current.Room.Name, fallbackTitle), resolvedRoomID, nil
}

func (s *Service) broadcastResync(ctx context.Context, request Request) {
	if s.events == nil || strings.TrimSpace(request.SessionKey) == "" {
		return
	}
	data := map[string]any{
		"reason": "title_generated",
	}
	if roomID := strings.TrimSpace(request.ConversationRoomID); roomID != "" {
		data["room_id"] = roomID
	}
	if conversationID := strings.TrimSpace(request.ConversationID); conversationID != "" {
		data["conversation_id"] = conversationID
	}
	event := protocol.NewEvent(protocol.EventTypeSessionResyncRequired, data)
	event.SessionKey = request.SessionKey
	if len(s.events.BroadcastEvent(ctx, request.SessionKey, event)) > 0 {
		s.logger.Warn("广播 session_resync_required 失败",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
		)
	}
}
