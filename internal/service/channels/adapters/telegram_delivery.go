package adapters

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

const telegramGeneralTopicID int64 = 1

func (c *TelegramChannel) SendDeliveryMessage(
	ctx context.Context,
	target channelcontract.DeliveryTarget,
	text string,
) (channelcontract.DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(c.token) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("telegram channel is not configured")
	}
	if strings.TrimSpace(target.To) == "" {
		return channelcontract.DeliveryResult{}, fmt.Errorf("telegram delivery target requires to")
	}

	parts := make([]channelmessage.ReceiptPart, 0)
	for _, chunk := range channeltransport.SplitText(strings.TrimSpace(text), 4000) {
		payload := map[string]any{
			"chat_id":                  target.To,
			"text":                     chunk,
			"disable_web_page_preview": true,
		}
		if err := applyTelegramSendThreadID(payload, target.ThreadID); err != nil {
			return channelcontract.DeliveryResult{}, err
		}
		var response telegramSendMessageResponse
		if err := channeltransport.DoJSONExpectSuccessDecode(
			ctx,
			c.client,
			http.MethodPost,
			strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/sendMessage",
			payload,
			nil,
			&response,
		); err != nil {
			return channelcontract.DeliveryResult{}, c.redactError(err)
		}
		if response.OK != nil && !*response.OK {
			description := strings.TrimSpace(response.Description)
			if description == "" {
				description = "ok=false"
			}
			return channelcontract.DeliveryResult{}, fmt.Errorf("telegram sendMessage failed: %s", description)
		}
		if response.Result.MessageID != 0 {
			parts = append(parts, channelmessage.TextPart(strconv.FormatInt(response.Result.MessageID, 10)))
		}
	}
	return channelcontract.NewDeliveryResult(normalized, channelmessage.NewReceipt(channelmessage.ReceiptParams{
		Channel:  channelcontract.ChannelTypeTelegram,
		Target:   target.To,
		ThreadID: target.ThreadID,
		Parts:    parts,
	})), nil
}

func (c *TelegramChannel) SendDeliveryTyping(ctx context.Context, target channelcontract.DeliveryTarget, active bool) error {
	if !active {
		return nil
	}
	if strings.TrimSpace(c.token) == "" {
		return fmt.Errorf("telegram channel is not configured")
	}
	if strings.TrimSpace(target.To) == "" {
		return fmt.Errorf("telegram typing target requires to")
	}

	payload := map[string]any{
		"chat_id": target.To,
		"action":  "typing",
	}
	if err := applyTelegramTypingThreadID(payload, target.ThreadID); err != nil {
		return err
	}
	if err := channeltransport.DoJSONExpectSuccess(
		ctx,
		c.client,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/bot"+c.token+"/sendChatAction",
		payload,
		nil,
	); err != nil {
		return c.redactError(err)
	}
	return nil
}

func applyTelegramSendThreadID(payload map[string]any, rawThreadID string) error {
	return applyTelegramThreadID(payload, rawThreadID, false)
}

func applyTelegramTypingThreadID(payload map[string]any, rawThreadID string) error {
	return applyTelegramThreadID(payload, rawThreadID, true)
}

func applyTelegramThreadID(payload map[string]any, rawThreadID string, includeGeneralTopic bool) error {
	if strings.TrimSpace(rawThreadID) == "" {
		return nil
	}
	threadID, err := strconv.ParseInt(strings.TrimSpace(rawThreadID), 10, 64)
	if err != nil {
		return fmt.Errorf("telegram thread_id is invalid: %w", err)
	}
	if threadID == telegramGeneralTopicID && !includeGeneralTopic {
		return nil
	}
	payload["message_thread_id"] = threadID
	return nil
}
