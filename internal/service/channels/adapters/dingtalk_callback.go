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
	return &channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeDingTalk,
		AccountID:    strings.TrimSpace(payload.ChatbotCorpID),
		ChatType:     normalizeDingTalkConversationType(payload.ConversationType),
		Ref:          ref,
		Content:      content,
		RoundID:      strings.TrimSpace(payload.MsgID),
		ReqID:        strings.TrimSpace(payload.MsgID),
		ExternalName: channelcontract.FirstNonEmpty(payload.ConversationTitle, payload.SenderNick),
		Delivery: &channelcontract.DeliveryTarget{
			Mode:      channelcontract.DeliveryModeExplicit,
			Channel:   channelcontract.ChannelTypeDingTalk,
			To:        deliveryTo,
			AccountID: strings.TrimSpace(payload.ChatbotCorpID),
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeDingTalk,
			Target:            ref,
			PlatformMessageID: strings.TrimSpace(payload.MsgID),
			SenderID:          channelcontract.FirstNonEmpty(payload.SenderStaffID, payload.SenderID),
			SenderName:        strings.TrimSpace(payload.SenderNick),
			ChatType:          normalizeDingTalkConversationType(payload.ConversationType),
			Text:              content,
			Metadata: map[string]string{
				"conversation_title": strings.TrimSpace(payload.ConversationTitle),
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
