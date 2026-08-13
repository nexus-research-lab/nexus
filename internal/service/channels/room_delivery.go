package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

const roomDeliverySource = automationDeliverySource

func (c *sessionDeliveryChannel) projectAutomationRoomResult(
	ctx context.Context,
	agentID string,
	parsed protocol.SessionKey,
	text string,
	delivery AutomationDeliveryContext,
) (*channelmessage.Receipt, error) {
	conversationID := strings.TrimSpace(parsed.ConversationID)
	if conversationID == "" {
		return nil, errors.New("room delivery requires conversation_id")
	}
	if c.roomHistory == nil {
		return nil, errors.New("session delivery 缺少 room history store")
	}
	ownerUserID := strings.TrimSpace(authctx.OwnerUserID(ctx))
	if ownerUserID == "" {
		return nil, errors.New("room delivery requires owner")
	}
	now := time.Now().UTC()
	roundID, assistantID, _ := automationDeliveryMessageIDs(delivery.RunID)
	assistantMessage := protocol.Message{
		"message_id":      assistantID,
		"session_key":     parsed.Raw,
		"conversation_id": conversationID,
		"agent_id":        strings.TrimSpace(agentID),
		"round_id":        roundID,
		"role":            "assistant",
		"timestamp":       now.UnixMilli(),
		"content":         []map[string]any{{"type": "text", "text": strings.TrimSpace(text)}},
		"stop_reason":     "end_turn",
		"is_complete":     true,
		"metadata":        automationDeliveryMetadata(delivery),
	}
	if err := c.roomHistory.AppendInlineMessage(ownerUserID, conversationID, assistantMessage); err != nil {
		return nil, err
	}
	c.broadcastRoomMessage(ctx, parsed.Raw, conversationID, strings.TrimSpace(agentID), roundID, assistantMessage)
	return automationProjectionReceipt(c.channelType, parsed.Raw, conversationID, assistantID), nil
}

func (c *sessionDeliveryChannel) sendRoomDeliveryText(
	ctx context.Context,
	agentID string,
	parsed protocol.SessionKey,
	sessionKey string,
	text string,
) (*channelmessage.Receipt, error) {
	if c.channelType != ChannelTypeWebSocket {
		return nil, fmt.Errorf("room delivery requires websocket channel: %s", c.channelType)
	}
	conversationID := strings.TrimSpace(parsed.ConversationID)
	if conversationID == "" {
		return nil, errors.New("room delivery requires conversation_id")
	}
	if c.roomHistory == nil {
		return nil, errors.New("session delivery 缺少 room history store")
	}

	now := time.Now().UTC()
	ownerUserID := strings.TrimSpace(authctx.OwnerUserID(ctx))
	if ownerUserID == "" {
		return nil, errors.New("room delivery requires owner")
	}
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
	if err := c.roomHistory.AppendInlineMessage(
		ownerUserID,
		conversationID,
		assistantMessage,
	); err != nil {
		return nil, err
	}

	c.broadcastRoomMessage(ctx, sessionKey, conversationID, strings.TrimSpace(agentID), roundID, assistantMessage)
	return channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  c.channelType,
		Target:   sessionKey,
		ThreadID: conversationID,
		Parts: []channelmessage.ReceiptPart{
			channelmessage.TextPart(stringValue(assistantMessage["message_id"])),
		},
	}), nil
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
	event.RoundID = roundID
	c.permission.BroadcastEvent(ctx, sessionKey, event)
}
