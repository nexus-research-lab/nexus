package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomDeliverySource = "automation_delivery"

func (c *sessionDeliveryChannel) sendRoomDeliveryText(
	ctx context.Context,
	agentID string,
	parsed protocol.SessionKey,
	sessionKey string,
	text string,
) error {
	if c.channelType != ChannelTypeWebSocket {
		return fmt.Errorf("room delivery requires websocket channel: %s", c.channelType)
	}
	conversationID := strings.TrimSpace(parsed.ConversationID)
	if conversationID == "" {
		return errors.New("room delivery requires conversation_id")
	}
	if c.roomHistory == nil {
		return errors.New("session delivery 缺少 room history store")
	}

	now := time.Now().UTC()
	roundID := c.idFactory("delivery_round")
	assistantMessage := protocol.Message{
		"message_id":      c.idFactory("assistant"),
		"session_key":     sessionKey,
		"conversation_id": conversationID,
		"agent_id":        strings.TrimSpace(agentID),
		"round_id":        roundID,
		"role":            "assistant",
		"timestamp":       now.UnixMilli(),
		"content": []map[string]any{
			{
				"type": "text",
				"text": strings.TrimSpace(text),
			},
		},
		"stop_reason": "end_turn",
		"is_complete": true,
		"metadata": map[string]any{
			"source": roomDeliverySource,
		},
	}
	if err := c.roomHistory.AppendInlineMessage(conversationID, assistantMessage); err != nil {
		return err
	}

	c.broadcastRoomMessage(ctx, sessionKey, conversationID, strings.TrimSpace(agentID), roundID, assistantMessage)
	return nil
}

func (c *sessionDeliveryChannel) broadcastRoomMessage(
	ctx context.Context,
	sessionKey string,
	conversationID string,
	agentID string,
	roundID string,
	message protocol.Message,
) {
	if c.permission == nil {
		return
	}
	event := protocol.NewEvent(protocol.EventTypeMessage, message)
	event.DeliveryMode = "durable"
	event.SessionKey = sessionKey
	event.ConversationID = conversationID
	event.AgentID = agentID
	event.MessageID = strings.TrimSpace(stringValue(message["message_id"]))
	event.CausedBy = roundID
	c.permission.BroadcastEvent(ctx, sessionKey, event)
}
