package adapters

import (
	"encoding/json"
	"strings"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

// DecodeDingTalkIngressCallback 将钉钉 HTTP/Webhook 机器人消息转换成统一通道入口请求。
func DecodeDingTalkIngressCallback(raw []byte) (*channelcontract.IngressRequest, string, error) {
	var payload struct {
		ConversationID     string `json:"conversationId"`
		OpenConversationID string `json:"openConversationId"`
		ConversationType   string `json:"conversationType"`
		ConversationTitle  string `json:"conversationTitle"`
		ChatbotCorpID      string `json:"chatbotCorpId"`
		SessionWebhook     string `json:"sessionWebhook"`
		SenderStaffID      string `json:"senderStaffId"`
		SenderID           string `json:"senderId"`
		SenderNick         string `json:"senderNick"`
		MsgID              string `json:"msgId"`
		MsgType            string `json:"msgtype"`
		Text               struct {
			Content string `json:"content"`
		} `json:"text"`
		Content any `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, "", err
	}
	content := channelcontract.FirstNonEmpty(payload.Text.Content, dingTalkContentText(payload.Content))
	if content == "" {
		return nil, "empty_text", nil
	}
	ref := channelcontract.FirstNonEmpty(payload.OpenConversationID, payload.ConversationID, payload.SenderStaffID, payload.SenderID)
	if ref == "" {
		return nil, "empty_ref", nil
	}
	deliveryTo := channelcontract.FirstNonEmpty(payload.SessionWebhook, payload.OpenConversationID, payload.ConversationID, ref)
	accountID := strings.TrimSpace(payload.ChatbotCorpID)
	messageID := strings.TrimSpace(payload.MsgID)
	chatType := normalizeDingTalkConversationType(payload.ConversationType)
	senderName := strings.TrimSpace(payload.SenderNick)
	conversationTitle := strings.TrimSpace(payload.ConversationTitle)
	return &channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeDingTalk,
		AccountID:    accountID,
		ChatType:     chatType,
		Ref:          ref,
		Content:      content,
		RoundID:      messageID,
		ReqID:        messageID,
		ExternalName: channelcontract.FirstNonEmpty(payload.ConversationTitle, payload.SenderNick),
		Delivery: &channelcontract.DeliveryTarget{
			Mode:      channelcontract.DeliveryModeExplicit,
			Channel:   channelcontract.ChannelTypeDingTalk,
			To:        deliveryTo,
			AccountID: accountID,
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeDingTalk,
			Target:            ref,
			PlatformMessageID: messageID,
			SenderID:          channelcontract.FirstNonEmpty(payload.SenderStaffID, payload.SenderID),
			SenderName:        senderName,
			ChatType:          chatType,
			Text:              content,
			Metadata: map[string]string{
				"conversation_title": conversationTitle,
			},
		}),
	}, "", nil
}

func dingTalkContentText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if text, ok := typed["content"].(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}
