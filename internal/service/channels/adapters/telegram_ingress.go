package adapters

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

func (c *TelegramChannel) handleUpdate(ctx context.Context, update telegramUpdate) error {
	message := update.Message
	edited := false
	if message == nil {
		message = update.EditedMessage
		edited = message != nil
	}
	if message == nil || message.From == nil || message.From.IsBot {
		return nil
	}

	content := channelcontract.FirstNonEmpty(message.Text, message.Caption)
	if content == "" {
		return nil
	}

	ingress := c.currentIngress()
	if ingress == nil {
		c.loggerFor(ctx).Warn("Telegram 入站消息缺少处理器",
			"owner_user_id", c.ownerUserID,
			"update_id", update.UpdateID,
		)
		return &channelcontract.RetryableIngressError{Err: errors.New("Telegram 入站消息缺少处理器")}
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
	// update_id 在 Bot 范围内唯一，message_id 仅在聊天内唯一。
	reqID := "update:" + strconv.Itoa(update.UpdateID)
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
		RoundID:     "telegram_" + strconv.Itoa(update.UpdateID),
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
			return nil
		}
		c.loggerFor(ctx).Warn("Telegram 入站消息处理失败",
			"owner_user_id", c.ownerUserID,
			"chat_type", chatType,
			"ref", ref,
			"thread_id", threadID,
			"message_id", messageID,
			"err", err,
		)
		return err
	}
	return nil
}

// reportIngressFailure 只尝试发送结果提示；通知失败不能重新执行原消息或回退消费游标。
func (c *TelegramChannel) reportIngressFailure(ctx context.Context, update telegramUpdate, cause error) {
	message := update.Message
	if message == nil {
		message = update.EditedMessage
	}
	if message == nil {
		return
	}
	notice := "⚠️ 未能确认这条消息是否处理成功，请先查看 Nexus 中的会话结果，避免重复执行。"
	var retryable *channelcontract.RetryableIngressError
	if errors.As(cause, &retryable) {
		notice = "⚠️ 本次接收失败，请检查 Nexus 中是否已有会话结果，再决定是否重新发送。"
	}
	notice += "\n消息编号：" + strconv.Itoa(message.MessageID)
	c.loggerFor(ctx).Warn("Telegram 入站事件处理结束，继续接收后续消息", "update_id", update.UpdateID, "err", c.redactError(cause))
	noticeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	target := channelcontract.DeliveryTarget{Mode: channelcontract.DeliveryModeExplicit, Channel: channelcontract.ChannelTypeTelegram, To: strconv.FormatInt(message.Chat.ID, 10)}
	if message.MessageThreadID != 0 {
		target.ThreadID = strconv.Itoa(message.MessageThreadID)
	}
	if _, err := c.SendDeliveryMessage(noticeCtx, target, notice); err != nil {
		c.loggerFor(ctx).Warn("Telegram 入站失败提示未确认送达", "update_id", update.UpdateID, "err", c.redactError(err))
	}
}
