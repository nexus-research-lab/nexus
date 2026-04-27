package chat

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) broadcastUserRoundMarker(
	ctx context.Context,
	sessionValue protocol.Session,
	roundID string,
	content string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
) {
	message := protocol.Message{
		"message_id":  strings.TrimSpace(roundID),
		"session_key": sessionValue.SessionKey,
		"agent_id":    sessionValue.AgentID,
		"round_id":    strings.TrimSpace(roundID),
		"role":        "user",
		"content":     strings.TrimSpace(content),
		"timestamp":   time.Now().UnixMilli(),
	}
	if strings.TrimSpace(string(deliveryPolicy)) != "" {
		message["delivery_policy"] = string(protocol.NormalizeChatDeliveryPolicy(string(deliveryPolicy)))
	}
	event := protocol.NewEvent(protocol.EventTypeMessage, message)
	event.DeliveryMode = "durable"
	event.SessionKey = sessionValue.SessionKey
	event.AgentID = sessionValue.AgentID
	event.MessageID = strings.TrimSpace(roundID)
	s.permission.BroadcastEvent(ctx, sessionValue.SessionKey, event)
}

func (s *Service) broadcastGuidanceMessage(
	ctx context.Context,
	sessionValue protocol.Session,
	targetRoundID string,
	sourceRoundID string,
	content string,
) {
	message := buildGuidanceMessage(sessionValue, targetRoundID, sourceRoundID, content, time.Now().UnixMilli())
	event := protocol.NewEvent(protocol.EventTypeMessage, message)
	event.DeliveryMode = "ephemeral"
	event.SessionKey = sessionValue.SessionKey
	event.AgentID = sessionValue.AgentID
	event.MessageID = normalizeString(message["message_id"])
	event.CausedBy = strings.TrimSpace(sourceRoundID)
	s.permission.BroadcastEvent(ctx, sessionValue.SessionKey, event)
}
