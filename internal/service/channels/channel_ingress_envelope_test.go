package channels

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestDiscordBuildIngressRequestIncludesMessageEnvelope(t *testing.T) {
	t.Parallel()

	channel := newDiscordChannel("token", nil).WithOwner("owner-a")
	request, err := channel.buildIngressRequest(&discordgo.Session{}, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "discord-message-1",
			ChannelID: "discord-dm-channel-1",
			Author: &discordgo.User{
				ID:       "discord-user-1",
				Username: "alice",
			},
			ReferencedMessage: &discordgo.Message{ID: "discord-message-0"},
		},
	}, "检查今天的任务")
	if err != nil {
		t.Fatalf("构造 Discord ingress 失败: %v", err)
	}

	if request.OwnerUserID != "owner-a" ||
		request.Channel != ChannelTypeDiscord ||
		request.ChatType != "dm" ||
		request.Ref != "discord-user-1" ||
		request.RoundID != "discord-message-1" ||
		request.ReqID != "discord-message-1" {
		t.Fatalf("Discord ingress 基础字段不正确: %+v", request)
	}
	if request.Message == nil ||
		request.Message.Channel != ChannelTypeDiscord ||
		request.Message.Target != "discord-user-1" ||
		request.Message.PlatformMessageID != "discord-message-1" ||
		request.Message.ReplyToID != "discord-message-0" ||
		request.Message.SenderID != "discord-user-1" ||
		request.Message.SenderName != "alice" ||
		request.Message.Text != "检查今天的任务" {
		t.Fatalf("Discord ingress envelope 不正确: %+v", request.Message)
	}
}

func TestDecodeDingTalkIngressCallbackIncludesMessageEnvelope(t *testing.T) {
	t.Parallel()

	request, ignored, err := DecodeDingTalkIngressCallback([]byte(`{
		"openConversationId": "cid-1",
		"conversationType": "2",
		"conversationTitle": "日报群",
		"chatbotCorpId": "corp-1",
		"sessionWebhook": "https://dingtalk.test/session-webhook",
		"senderStaffId": "staff-1",
		"senderId": "sender-1",
		"senderNick": "Alice",
		"msgId": "ding-message-1",
		"msgtype": "text",
		"text": {"content": "检查本周日报任务"}
	}`))
	if err != nil {
		t.Fatalf("解析钉钉 ingress 失败: %v", err)
	}
	if ignored != "" || request == nil {
		t.Fatalf("钉钉文本消息不应被忽略: request=%+v ignored=%s", request, ignored)
	}
	if request.Delivery == nil || request.Delivery.To != "https://dingtalk.test/session-webhook" {
		t.Fatalf("钉钉回投应优先使用 sessionWebhook: %+v", request.Delivery)
	}
	if request.Message == nil ||
		request.Message.Channel != ChannelTypeDingTalk ||
		request.Message.Target != "cid-1" ||
		request.Message.PlatformMessageID != "ding-message-1" ||
		request.Message.SenderID != "staff-1" ||
		request.Message.SenderName != "Alice" ||
		request.Message.ChatType != "group" ||
		request.Message.Text != "检查本周日报任务" ||
		request.Message.Metadata["conversation_title"] != "日报群" {
		t.Fatalf("钉钉 ingress envelope 不正确: %+v", request.Message)
	}
}
