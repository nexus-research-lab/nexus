package adapters

import (
	"testing"

	"github.com/bwmarrin/discordgo"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
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
