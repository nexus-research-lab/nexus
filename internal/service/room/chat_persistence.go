package room

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) persistSharedInlineMessage(conversationID string, message protocol.Message) error {
	if err := s.roomHistory.AppendInlineMessage(conversationID, message); err != nil {
		return err
	}
	s.touchSharedConversationActivity(context.Background(), conversationID, roomMessageActivityTime(message))
	return nil
}

func (s *RealtimeService) persistSharedDurableMessage(
	conversationID string,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	if slot == nil || !protocol.IsTranscriptNativeMessage(protocol.Message(message)) {
		return s.persistSharedInlineMessage(conversationID, message)
	}
	if err := s.roomHistory.AppendTranscriptReference(
		conversationID,
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		message,
	); err != nil {
		return err
	}
	s.touchSharedConversationActivity(context.Background(), conversationID, roomMessageActivityTime(message))
	return nil
}

func (s *RealtimeService) touchSharedConversationActivity(ctx context.Context, conversationID string, activityAt time.Time) {
	if s == nil || s.rooms == nil {
		return
	}
	if activityAt.IsZero() {
		activityAt = time.Now().UTC()
	}
	if err := s.rooms.TouchConversationActivity(ctx, conversationID, activityAt); err != nil {
		s.loggerFor(ctx).Error("更新 Room conversation 活动时间失败",
			"conversation_id", conversationID,
			"activity_at", activityAt,
			"err", err,
		)
	}
}

func roomMessageActivityTime(message protocol.Message) time.Time {
	if len(message) == 0 {
		return time.Now().UTC()
	}
	return roomTimestampActivityTime(message["timestamp"])
}

func roomTimestampActivityTime(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC()
	case json.Number:
		return roomUnixMilliActivityTime(typed.String())
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" {
			return time.Now().UTC()
		}
		if parsed, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
			return parsed.UTC()
		}
		if parsed, err := time.Parse(time.RFC3339, normalized); err == nil {
			return parsed.UTC()
		}
		return roomUnixMilliActivityTime(normalized)
	case int:
		return time.UnixMilli(int64(typed)).UTC()
	case int64:
		return time.UnixMilli(typed).UTC()
	case int32:
		return time.UnixMilli(int64(typed)).UTC()
	case float64:
		return time.UnixMilli(int64(typed)).UTC()
	case float32:
		return time.UnixMilli(int64(typed)).UTC()
	default:
		return time.Now().UTC()
	}
}

func roomUnixMilliActivityTime(value string) time.Time {
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.UnixMilli(parsed).UTC()
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return time.UnixMilli(int64(parsed)).UTC()
	}
	return time.Now().UTC()
}
