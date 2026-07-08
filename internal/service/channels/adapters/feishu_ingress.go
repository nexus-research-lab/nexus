package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

func (c *FeishuChannel) handleSDKMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil {
		return nil
	}
	raw, err := feishuSDKEventRaw(event.EventReq, event)
	if err != nil {
		return err
	}
	callback, err := DecodeFeishuIngressCallback(raw)
	if err != nil {
		return err
	}
	return c.acceptDecodedIngress(ctx, callback)
}

func (c *FeishuChannel) handleSDKReaction(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
	if event == nil {
		return nil
	}
	raw, err := feishuSDKEventRaw(event.EventReq, event)
	if err != nil {
		return err
	}
	callback, err := DecodeFeishuIngressCallback(raw)
	if err != nil {
		return err
	}
	return c.acceptDecodedIngress(ctx, callback)
}

func feishuSDKEventRaw(eventReq *larkevent.EventReq, fallback any) ([]byte, error) {
	if eventReq != nil && len(bytes.TrimSpace(eventReq.Body)) > 0 {
		return bytes.TrimSpace(eventReq.Body), nil
	}
	return json.Marshal(fallback)
}

func (c *FeishuChannel) acceptDecodedIngress(ctx context.Context, callback FeishuIngressCallback) error {
	ingress := c.currentIngress()
	if ingress == nil || callback.Request == nil {
		return nil
	}
	callback.Request.OwnerUserID = c.ownerUserID
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, *callback.Request); err != nil {
		if IsPairingApprovalRequired(err) {
			if callback.Request.Delivery != nil {
				if notice := PairingApprovalNoticeText(err); notice != "" {
					_, _ = c.SendDeliveryMessage(requestCtx, *callback.Request.Delivery, notice)
				}
			}
			return nil
		}
		return err
	}
	return nil
}

// DecodeFeishuIngressCallback 将飞书事件订阅回调转换成统一通道入口请求。
func DecodeFeishuIngressCallback(raw []byte) (FeishuIngressCallback, error) {
	if _, encrypted, err := FeishuEncryptEnvelope(raw); err == nil && encrypted {
		return FeishuIngressCallback{}, errors.New("encrypted feishu callback requires configured encrypt_key")
	}
	var payload feishuEventCallbackPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return FeishuIngressCallback{}, err
	}
	callback := FeishuIngressCallback{
		Challenge: strings.TrimSpace(payload.Challenge),
		AppID:     strings.TrimSpace(payload.Header.AppID),
		Token:     channelcontract.FirstNonEmpty(payload.Header.Token, payload.Token),
	}
	if callback.AppID == "" {
		callback.AppID = strings.TrimSpace(payload.Event.AppID)
	}
	if callback.Challenge != "" || strings.EqualFold(strings.TrimSpace(payload.Type), "url_verification") {
		return callback, nil
	}

	eventType := strings.TrimSpace(channelcontract.FirstNonEmpty(payload.Header.EventType, payload.Type))
	switch eventType {
	case "im.message.receive_v1":
		callback.Request = decodeFeishuMessageIngress(payload, &callback)
	case "im.message.reaction.created_v1":
		callback.Request = decodeFeishuReactionIngress(payload, &callback)
	case "im.message.message_read_v1", "im.message.reaction.deleted_v1":
		callback.IgnoredReason = "ignored_event_type"
	default:
		callback.IgnoredReason = "unsupported_event_type"
		return callback, nil
	}
	if callback.Request != nil && callback.Request.AccountID == "" {
		callback.Request.AccountID = strings.TrimSpace(callback.AppID)
	}
	return callback, nil
}

func decodeFeishuMessageIngress(payload feishuEventCallbackPayload, callback *FeishuIngressCallback) *channelcontract.IngressRequest {
	if isFeishuBotSender(payload.Event.Sender.SenderType) {
		callback.IgnoredReason = "bot_message"
		return nil
	}
	message := payload.Event.Message
	messageID := strings.TrimSpace(message.MessageID)
	chatID := strings.TrimSpace(message.ChatID)
	appID := strings.TrimSpace(callback.AppID)
	if messageID == "" && chatID == "" {
		callback.IgnoredReason = "empty_message"
		return nil
	}
	content := feishuMessageText(message)
	if content == "" {
		callback.IgnoredReason = "empty_text"
		return nil
	}

	ref, accountID := feishuMessageRef(chatID, payload.Event.Sender.SenderID)
	if ref == "" {
		callback.IgnoredReason = "empty_ref"
		return nil
	}
	threadID := channelcontract.FirstNonEmpty(message.ThreadID, message.RootID)
	reqID := channelcontract.FirstNonEmpty(messageID, payload.Header.EventID)
	chatType := normalizeFeishuChatType(message.ChatType)
	senderID, _ := feishuSenderRef(payload.Event.Sender.SenderID)

	return &channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeFeishu,
		AccountID:    appID,
		ChatType:     chatType,
		Ref:          ref,
		ThreadID:     threadID,
		Content:      content,
		RoundID:      channelcontract.FirstNonEmpty(payload.Header.EventID, messageID),
		ReqID:        reqID,
		ExternalName: chatID,
		Delivery: &channelcontract.DeliveryTarget{
			Mode:      channelcontract.DeliveryModeExplicit,
			Channel:   channelcontract.ChannelTypeFeishu,
			To:        ref,
			AccountID: accountID,
			ThreadID:  messageID,
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeFeishu,
			Target:            ref,
			PlatformMessageID: reqID,
			ThreadID:          threadID,
			SenderID:          senderID,
			SenderName:        chatID,
			ChatType:          chatType,
			Text:              content,
		}),
	}
}

func decodeFeishuReactionIngress(payload feishuEventCallbackPayload, callback *FeishuIngressCallback) *channelcontract.IngressRequest {
	emoji := strings.TrimSpace(payload.Event.ReactionType.EmojiType)
	messageID := strings.TrimSpace(payload.Event.MessageID)
	senderID, _ := feishuSenderRef(payload.Event.UserID)
	if emoji == "" || messageID == "" || senderID == "" {
		callback.IgnoredReason = "empty_reaction"
		return nil
	}
	if isFeishuBotSender(payload.Event.OperatorType) {
		callback.IgnoredReason = "bot_reaction"
		return nil
	}
	if strings.EqualFold(emoji, "Typing") {
		callback.IgnoredReason = "typing_reaction"
		return nil
	}

	chatID := strings.TrimSpace(payload.Event.ChatID)
	appID := strings.TrimSpace(callback.AppID)
	chatType := normalizeFeishuChatType(payload.Event.ChatType)
	reactionText := fmt.Sprintf("[reacted with %s to message %s]", emoji, messageID)
	ref, accountID := feishuReactionRef(chatID, senderID)
	if ref == "" {
		callback.IgnoredReason = "empty_ref"
		return nil
	}

	threadID := channelcontract.FirstNonEmpty(payload.Event.ThreadID, payload.Event.RootID)
	reqID := strings.Join([]string{
		messageID,
		"reaction",
		emoji,
		channelcontract.FirstNonEmpty(payload.Header.EventID, payload.Event.ActionTime),
	}, ":")
	return &channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeFeishu,
		AccountID:    appID,
		ChatType:     chatType,
		Ref:          ref,
		ThreadID:     threadID,
		Content:      reactionText,
		RoundID:      channelcontract.FirstNonEmpty(payload.Header.EventID, reqID),
		ReqID:        reqID,
		ExternalName: chatID,
		Delivery: &channelcontract.DeliveryTarget{
			Mode:      channelcontract.DeliveryModeExplicit,
			Channel:   channelcontract.ChannelTypeFeishu,
			To:        ref,
			AccountID: accountID,
			ThreadID:  messageID,
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeFeishu,
			Target:            ref,
			PlatformMessageID: messageID,
			ThreadID:          threadID,
			SenderID:          senderID,
			SenderName:        chatID,
			ChatType:          chatType,
			Text:              reactionText,
			ReplyToID:         messageID,
			Metadata: map[string]string{
				"reaction": emoji,
				"event_id": channelcontract.FirstNonEmpty(payload.Header.EventID, payload.Event.ActionTime),
			},
		}),
	}
}

func feishuMessageText(message feishuEventMessage) string {
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return ""
	}
	messageType := strings.TrimSpace(message.MessageType)
	if strings.EqualFold(messageType, "text") || messageType == "" {
		var textPayload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &textPayload); err == nil {
			if text := strings.TrimSpace(textPayload.Text); text != "" {
				return text
			}
		}
	}
	return content
}

func feishuSenderRef(senderID feishuEventSenderID) (string, string) {
	if value := strings.TrimSpace(senderID.OpenID); value != "" {
		return value, "open_id"
	}
	if value := strings.TrimSpace(senderID.UserID); value != "" {
		return value, "user_id"
	}
	if value := strings.TrimSpace(senderID.UnionID); value != "" {
		return value, "union_id"
	}
	return "", ""
}

func feishuMessageRef(chatID string, senderID feishuEventSenderID) (string, string) {
	if ref := strings.TrimSpace(chatID); ref != "" {
		return ref, "chat_id"
	}
	return feishuSenderRef(senderID)
}

func feishuReactionRef(chatID string, senderID string) (string, string) {
	if ref := strings.TrimSpace(chatID); ref != "" {
		return ref, "chat_id"
	}
	if senderID != "" {
		return senderID, "open_id"
	}
	return "", ""
}

func normalizeFeishuChatType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "group", "chat":
		return "group"
	case "p2p", "private", "dm":
		return "dm"
	default:
		return "group"
	}
}

func isFeishuBotSender(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "app", "bot":
		return true
	default:
		return false
	}
}
