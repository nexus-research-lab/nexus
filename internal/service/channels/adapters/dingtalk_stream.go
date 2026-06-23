package adapters

import (
	"context"
	"fmt"
	"strings"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	dingchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dingclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
)

func (c *DingTalkChannel) Start(ctx context.Context) error {
	if c.clientID == "" || c.clientSecret == "" {
		return fmt.Errorf("dingtalk channel is not configured")
	}

	c.mu.Lock()
	if c.stream != nil {
		c.mu.Unlock()
		return nil
	}
	stream := dingclient.NewStreamClient(
		dingclient.WithAppCredential(dingclient.NewAppCredentialConfig(c.clientID, c.clientSecret)),
		dingclient.WithOpenApiHost(c.streamHost),
	)
	stream.RegisterChatBotCallbackRouter(c.handleStreamMessage)
	c.stream = stream
	c.mu.Unlock()

	if err := stream.Start(ctx); err != nil {
		c.mu.Lock()
		if c.stream == stream {
			c.stream = nil
		}
		c.mu.Unlock()
		stream.Close()
		return err
	}
	return nil
}

func (c *DingTalkChannel) Stop(context.Context) error {
	c.mu.Lock()
	stream := c.stream
	c.stream = nil
	c.mu.Unlock()
	if stream != nil {
		stream.Close()
	}
	return nil
}

func (c *DingTalkChannel) handleStreamMessage(ctx context.Context, data *dingchatbot.BotCallbackDataModel) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	content := strings.TrimSpace(data.Text.Content)
	if content == "" {
		return nil, nil
	}

	ingress := c.currentIngress()
	if ingress == nil {
		return nil, nil
	}

	chatType := normalizeDingTalkConversationType(data.ConversationType)
	ref := channelcontract.FirstNonEmpty(data.ConversationId, data.SenderStaffId, data.SenderId)
	if ref == "" {
		return nil, nil
	}
	deliveryTo := channelcontract.FirstNonEmpty(data.SessionWebhook, data.ConversationId, ref)
	delivery := &channelcontract.DeliveryTarget{
		Mode:      channelcontract.DeliveryModeExplicit,
		Channel:   channelcontract.ChannelTypeDingTalk,
		To:        deliveryTo,
		AccountID: strings.TrimSpace(data.ChatbotCorpId),
	}

	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := ingress.Accept(requestCtx, channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeDingTalk,
		OwnerUserID:  c.ownerUserID,
		AccountID:    strings.TrimSpace(c.clientID),
		ChatType:     chatType,
		Ref:          ref,
		Content:      content,
		RoundID:      strings.TrimSpace(data.MsgId),
		ReqID:        strings.TrimSpace(data.MsgId),
		ExternalName: channelcontract.FirstNonEmpty(data.ConversationTitle, data.SenderNick),
		Delivery:     delivery,
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeDingTalk,
			Target:            ref,
			PlatformMessageID: strings.TrimSpace(data.MsgId),
			SenderID:          channelcontract.FirstNonEmpty(data.SenderStaffId, data.SenderId),
			SenderName:        strings.TrimSpace(data.SenderNick),
			ChatType:          chatType,
			Text:              content,
			Metadata: map[string]string{
				"conversation_title": strings.TrimSpace(data.ConversationTitle),
			},
		}),
	}); err != nil {
		if IsPairingApprovalRequired(err) {
			if notice := PairingApprovalNoticeText(err); notice != "" {
				_, _ = c.SendDeliveryMessage(ctx, *delivery, notice)
			}
			return []byte(""), nil
		}
		if strings.TrimSpace(data.SessionWebhook) != "" {
			if notifyErr := c.sendSessionWebhookText(ctx, data.SessionWebhook, "DingTalk 消息处理失败: "+TruncateError(err)); notifyErr == nil {
				return []byte(""), nil
			}
		}
		return nil, err
	}
	return []byte(""), nil
}
