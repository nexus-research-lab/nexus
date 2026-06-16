package adapters

import (
	"context"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestDiscordBuildIngressRequestIncludesMessageEnvelope(t *testing.T) {
	t.Parallel()

	channel := NewDiscordChannel("token", nil).WithOwner("owner-a")
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
		request.Channel != channelcontract.ChannelTypeDiscord ||
		request.ChatType != "dm" ||
		request.Ref != "discord-user-1" ||
		request.RoundID != "discord-message-1" ||
		request.ReqID != "discord-message-1" {
		t.Fatalf("Discord ingress 基础字段不正确: %+v", request)
	}
	if request.Message == nil ||
		request.Message.Channel != channelcontract.ChannelTypeDiscord ||
		request.Message.Target != "discord-user-1" ||
		request.Message.PlatformMessageID != "discord-message-1" ||
		request.Message.ReplyToID != "discord-message-0" ||
		request.Message.SenderID != "discord-user-1" ||
		request.Message.SenderName != "alice" ||
		request.Message.Text != "检查今天的任务" {
		t.Fatalf("Discord ingress envelope 不正确: %+v", request.Message)
	}
}

func TestDiscordBuildIngressRequestKeepsDMUsersSeparate(t *testing.T) {
	t.Parallel()

	channel := NewDiscordChannel("token", nil).WithOwner("owner-a")
	users := []struct {
		id        string
		channelID string
	}{
		{id: "discord-user-1", channelID: "discord-dm-channel-1"},
		{id: "discord-user-2", channelID: "discord-dm-channel-2"},
	}
	seenRefs := map[string]bool{}
	for _, user := range users {
		request, err := channel.buildIngressRequest(&discordgo.Session{}, &discordgo.MessageCreate{
			Message: &discordgo.Message{
				ID:        "message-" + user.id,
				ChannelID: user.channelID,
				Author:    &discordgo.User{ID: user.id},
			},
		}, "hello")
		if err != nil {
			t.Fatalf("构造 Discord ingress 失败 user=%s err=%v", user.id, err)
		}
		if request.ChatType != "dm" || request.Ref != user.id {
			t.Fatalf("Discord DM session ref 应使用外部用户 ID: %+v", request)
		}
		if request.Delivery == nil || request.Delivery.To != user.channelID {
			t.Fatalf("Discord DM 回投目标应使用私聊 channel_id: %+v", request.Delivery)
		}
		if seenRefs[request.Ref] {
			t.Fatalf("不同 Discord 用户不应复用 session ref: %+v", request)
		}
		seenRefs[request.Ref] = true
	}
}

func TestTelegramChannelHandleUpdateKeepsDMUsersSeparate(t *testing.T) {
	t.Parallel()

	channel := NewTelegramChannel("token", nil).WithOwner("owner-a")
	ingress := &recordingIngressAcceptor{}
	channel.SetIngress(ingress)
	for _, userID := range []int64{101, 202} {
		channel.handleUpdate(context.Background(), telegramUpdate{
			Message: &telegramMessage{
				MessageID: int(userID),
				Text:      "hello",
				From:      &telegramUser{ID: userID},
				Chat:      telegramChat{ID: userID, Type: "private"},
			},
		})
	}
	if len(ingress.requests) != 2 {
		t.Fatalf("两个 Telegram 私聊用户都应进入 ingress: %+v", ingress.requests)
	}
	seenRefs := map[string]bool{}
	for _, request := range ingress.requests {
		if request.OwnerUserID != "owner-a" || request.ChatType != "dm" || request.Ref == "" {
			t.Fatalf("Telegram DM ingress 基础字段不正确: %+v", request)
		}
		if request.Delivery == nil || request.Delivery.To != request.Ref {
			t.Fatalf("Telegram DM 回投目标不正确: %+v", request.Delivery)
		}
		if seenRefs[request.Ref] {
			t.Fatalf("不同 Telegram 私聊用户不应复用 session ref: %+v", ingress.requests)
		}
		seenRefs[request.Ref] = true
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
		request.Message.Channel != channelcontract.ChannelTypeDingTalk ||
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

func TestDecodeDingTalkIngressCallbackKeepsDirectUsersSeparate(t *testing.T) {
	t.Parallel()

	payloads := []string{
		`{"conversationType":"1","senderStaffId":"staff-1","senderNick":"Alice","msgId":"ding-message-1","msgtype":"text","text":{"content":"hello"}}`,
		`{"conversationType":"1","senderStaffId":"staff-2","senderNick":"Bob","msgId":"ding-message-2","msgtype":"text","text":{"content":"hello"}}`,
	}
	seenRefs := map[string]bool{}
	for _, payload := range payloads {
		request, ignored, err := DecodeDingTalkIngressCallback([]byte(payload))
		if err != nil {
			t.Fatalf("解析钉钉私聊 ingress 失败: %v", err)
		}
		if ignored != "" || request == nil {
			t.Fatalf("钉钉私聊文本消息不应被忽略: request=%+v ignored=%s", request, ignored)
		}
		if request.ChatType != "dm" || request.Ref == "" {
			t.Fatalf("钉钉私聊 session ref 不正确: %+v", request)
		}
		if request.Delivery == nil || request.Delivery.To != request.Ref {
			t.Fatalf("钉钉私聊回投目标不正确: %+v", request.Delivery)
		}
		if seenRefs[request.Ref] {
			t.Fatalf("不同钉钉私聊用户不应复用 session ref: %+v", request)
		}
		seenRefs[request.Ref] = true
	}
}
