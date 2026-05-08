package channels

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestChannelCatalogMarksAllIMChannelsPlanned(t *testing.T) {
	for _, item := range channelCatalog() {
		if item.RuntimeStatus != "planned" {
			t.Fatalf("%s 应标记为未上线，实际 runtime_status=%s", item.ChannelType, item.RuntimeStatus)
		}
		if !strings.Contains(item.Description, "未上线") {
			t.Fatalf("%s 描述应展示未上线，实际 description=%s", item.ChannelType, item.Description)
		}
	}
}

func TestControlServiceRejectsPlannedChannelConfig(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	cases := []struct {
		name        string
		channelType string
		config      map[string]string
		credentials map[string]string
	}{
		{
			name:        "dingtalk",
			channelType: ChannelTypeDingTalk,
			config:      map[string]string{"client_id": "ding-client"},
			credentials: map[string]string{"client_secret": "ding-secret"},
		},
		{
			name:        "wechat",
			channelType: ChannelTypeWeChat,
		},
		{
			name:        "feishu",
			channelType: ChannelTypeFeishu,
			config:      map[string]string{"app_id": "cli_xxx"},
			credentials: map[string]string{"app_secret": "feishu-secret"},
		},
		{
			name:        "telegram",
			channelType: ChannelTypeTelegram,
			credentials: map[string]string{
				"bot_token": "token",
			},
		},
		{
			name:        "discord",
			channelType: ChannelTypeDiscord,
			config:      map[string]string{"application_id": "123"},
			credentials: map[string]string{
				"bot_token": "token",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.UpsertChannelConfig(context.Background(), "owner-a", tc.channelType, UpsertChannelConfigRequest{
				AgentID:     "agent-a",
				Config:      tc.config,
				Credentials: tc.credentials,
			})
			if err == nil || !strings.Contains(err.Error(), "消息渠道未上线") {
				t.Fatalf("未上线渠道应拒绝配置，实际 err=%v", err)
			}
		})
	}
}

func TestControlServiceExcludesPlannedChannelsFromSummaryCounts(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'telegram', 'agent-a', 'configured', '{}');
INSERT INTO im_pairings (pairing_id, owner_user_id, channel_type, chat_type, external_ref, agent_id, status, source)
VALUES ('pairing-a', 'owner-a', 'telegram', 'dm', 'chat-a', 'agent-a', 'active', 'manual');
`); err != nil {
		t.Fatalf("准备 IM 数据失败: %v", err)
	}

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	configured, err := service.CountConfiguredChannels(context.Background(), "owner-a")
	if err != nil {
		t.Fatalf("统计已配置渠道失败: %v", err)
	}
	if configured != 0 {
		t.Fatalf("未上线渠道不应计入已配置渠道数，实际 %d", configured)
	}

	activePairings, err := service.CountActivePairings(context.Background(), "owner-a")
	if err != nil {
		t.Fatalf("统计活跃配对失败: %v", err)
	}
	if activePairings != 0 {
		t.Fatalf("未上线渠道不应计入活跃配对数，实际 %d", activePairings)
	}
}
