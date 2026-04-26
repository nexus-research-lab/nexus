package chat

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/session"
)

func (s *Service) broadcastUserRoundMarker(
	ctx context.Context,
	sessionValue session.Session,
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
