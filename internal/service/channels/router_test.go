package channels

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestRouterRegisterAndStartLetsNewChannelAdoptReplaced(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	replaced := &recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", replaced); err != nil {
		t.Fatalf("注册旧通道失败: %v", err)
	}
	next := &adoptingDeliveryChannel{
		recordingDeliveryChannel: recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal},
	}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", next); err != nil {
		t.Fatalf("注册新通道失败: %v", err)
	}

	if next.adopted != replaced {
		t.Fatalf("新通道应接管旧通道: got=%T", next.adopted)
	}
	if replaced.stops != 0 {
		t.Fatalf("接管成功时旧通道不应被停止，stops=%d", replaced.stops)
	}
	if next.starts != 1 {
		t.Fatalf("新通道仍应启动，starts=%d", next.starts)
	}
}

func TestNewRouterHonorsChannelEnabledFlags(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(
		config.Config{
			DatabaseDriver:   "sqlite",
			DiscordEnabled:   false,
			DiscordBotToken:  "discord-token",
			TelegramEnabled:  false,
			TelegramBotToken: "telegram-token",
		},
		db,
		nil,
		nil,
	)

	if router.GetForOwner("", ChannelTypeDiscord) != nil {
		t.Fatal("DISCORD_ENABLED=false 时不应注册 discord 通道")
	}
	if router.GetForOwner("", ChannelTypeTelegram) != nil {
		t.Fatal("TELEGRAM_ENABLED=false 时不应注册 telegram 通道")
	}
	if router.GetForOwner("", ChannelTypeWebSocket) == nil {
		t.Fatal("websocket 通道不应受开关影响")
	}
	if router.GetForOwner("", ChannelTypeInternal) == nil {
		t.Fatal("internal 通道不应受开关影响")
	}
}
