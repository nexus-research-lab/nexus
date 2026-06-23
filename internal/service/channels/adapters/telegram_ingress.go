package adapters

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

func (c *TelegramChannel) handleUpdate(ctx context.Context, update telegramUpdate) {
	message := update.Message
	edited := false
	if message == nil {
		message = update.EditedMessage
		edited = message != nil
	}
	if message == nil || message.From == nil || message.From.IsBot {
		return
	}

	content := strings.TrimSpace(message.Text)
	if content == "" {
		content = strings.TrimSpace(message.Caption)
	}
	if content == "" {
		return
	}

	ingress := c.currentIngress()
	if ingress == nil {
		c.loggerFor(ctx).Warn("Telegram 入站消息缺少处理器",
			"owner_user_id", c.ownerUserID,
			"update_id", update.UpdateID,
		)
		return
	}

	chatType := "group"
	ref := strconv.FormatInt(message.Chat.ID, 10)
	threadID := ""
	delivery := &channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeTelegram,
		To:      strconv.FormatInt(message.Chat.ID, 10),
	}
	if strings.EqualFold(message.Chat.Type, "private") {
		chatType = "dm"
		ref = strconv.FormatInt(message.From.ID, 10)
	}
	if message.MessageThreadID != 0 {
		threadID = strconv.Itoa(message.MessageThreadID)
		delivery.ThreadID = threadID
	}

	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	messageID := strconv.Itoa(message.MessageID)
	reqID := messageID
	if edited {
		reqID = telegramEditedMessageReqID(messageID, update.UpdateID)
	}
	c.loggerFor(ctx).Debug("收到 Telegram 入站消息",
		"owner_user_id", c.ownerUserID,
		"chat_type", chatType,
		"ref", ref,
		"thread_id", threadID,
		"message_id", messageID,
		"edited", edited,
		"chars", len([]rune(content)),
	)
	if _, err := ingress.Accept(requestCtx, channelcontract.IngressRequest{
		Channel:     channelcontract.ChannelTypeTelegram,
		OwnerUserID: c.ownerUserID,
		AccountID:   AccountIDFromSecret("tg", c.token),
		ChatType:    chatType,
		Ref:         ref,
		ThreadID:    threadID,
		Content:     content,
		ReqID:       reqID,
		Delivery:    delivery,
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeTelegram,
			Target:            ref,
			PlatformMessageID: messageID,
			ThreadID:          threadID,
			SenderID:          strconv.FormatInt(message.From.ID, 10),
			ChatType:          chatType,
			Text:              content,
			Edited:            edited,
		}),
	}); err != nil {
		if IsPairingApprovalRequired(err) {
			if notice := PairingApprovalNoticeText(err); notice != "" {
				_, _ = c.SendDeliveryMessage(requestCtx, *delivery, notice)
			}
			return
		}
		c.loggerFor(ctx).Warn("Telegram 入站消息处理失败",
			"owner_user_id", c.ownerUserID,
			"chat_type", chatType,
			"ref", ref,
			"thread_id", threadID,
			"message_id", messageID,
			"err", err,
		)
		_, _ = c.SendDeliveryMessage(requestCtx, *delivery, "⚠️ Telegram 消息处理失败: "+TruncateError(err))
	}
}

func telegramEditedMessageReqID(messageID string, updateID int) string {
	trimmed := strings.TrimSpace(messageID)
	if updateID == 0 {
		return trimmed + ":edited"
	}
	return fmt.Sprintf("%s:edited:%d", trimmed, updateID)
}
