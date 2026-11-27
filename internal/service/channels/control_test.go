// control_test.go 覆盖渠道配置控制、变更通知、失败处理、配对管理、Agent 删除与个人微信状态存储测试。
package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

func TestControlServiceRejectsIncompleteChannelConfig(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	cases := []struct {
		name        string
		channelType string
		config      map[string]string
		credentials map[string]string
		want        string
	}{
		{
			name:        "dingtalk",
			channelType: ChannelTypeDingTalk,
			config:      map[string]string{"client_id": "ding-client"},
			want:        "client_secret is required",
		},
		{
			name:        "wechat",
			channelType: ChannelTypeWeChat,
			config:      map[string]string{"bot_id": "bot-1"},
			want:        "secret is required",
		},
		{
			name:        "telegram",
			channelType: ChannelTypeTelegram,
			want:        "bot_token is required",
		},
		{
			name:        "discord",
			channelType: ChannelTypeDiscord,
			config:      map[string]string{"application_id": "123"},
			want:        "bot_token is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.UpsertChannelConfig(context.Background(), "owner-a", tc.channelType, UpsertChannelConfigRequest{
				AgentID:     "agent-a",
				Config:      tc.config,
				Credentials: tc.credentials,
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("不完整渠道配置应拒绝，实际 err=%v want=%s", err, tc.want)
			}
		})
	}
}

func TestControlServiceAppliesOptionalRuntimeChannelConfig(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, router)
	ctx := context.Background()

	_, err := service.UpsertChannelConfig(ctx, "owner-a", ChannelTypeDingTalk, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"client_id":       "ding-client",
			"base_url":        "https://ding-api.test/",
			"stream_base_url": "https://ding-stream.test/",
		},
		Credentials: map[string]string{"client_secret": "ding-secret"},
	})
	if err != nil {
		t.Fatalf("配置钉钉失败: %v", err)
	}
	dingtalk, ok := router.GetForOwner("owner-a", ChannelTypeDingTalk).(*channeladapters.DingTalkChannel)
	if !ok || dingtalk.BaseURL() != "https://ding-api.test" || dingtalk.StreamHost() != "https://ding-stream.test" {
		t.Fatalf("钉钉运行时配置未生效: channel=%+v ok=%v", dingtalk, ok)
	}

	_, err = service.UpsertChannelConfig(ctx, "owner-a", ChannelTypeWeChat, UpsertChannelConfigRequest{
		AgentID:     "agent-a",
		Config:      map[string]string{"bot_id": "wechat-bot", "base_url": "wss://wecom.test/ws/"},
		Credentials: map[string]string{"secret": "wechat-secret"},
	})
	if err != nil {
		t.Fatalf("配置企业微信失败: %v", err)
	}
	wechat, ok := router.GetForOwner("owner-a", ChannelTypeWeChat).(*channeladapters.WeComBotChannel)
	if !ok || wechat.BaseURL() != "wss://wecom.test/ws" {
		t.Fatalf("企业微信运行时配置未生效: channel=%+v ok=%v", wechat, ok)
	}

	_, err = service.UpsertChannelConfig(ctx, "owner-a", ChannelTypeFeishu, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"app_id":          "cli_a",
			"connection_mode": "webhook",
			"base_url":        "https://feishu-api.test",
			"reply_in_thread": "true",
		},
		Credentials: map[string]string{"app_secret": "feishu-secret"},
	})
	if err != nil {
		t.Fatalf("配置飞书失败: %v", err)
	}
	feishu, ok := router.GetForOwner("owner-a", ChannelTypeFeishu).(*channeladapters.FeishuChannel)
	if !ok || feishu.ConnectionMode() != "webhook" || feishu.BaseURL() != "https://feishu-api.test" || !feishu.ReplyInThread() {
		t.Fatalf("飞书运行时配置未生效: channel=%+v ok=%v", feishu, ok)
	}

	_, err = service.UpsertChannelConfig(ctx, "owner-a", ChannelTypeTelegram, UpsertChannelConfigRequest{
		AgentID:     "agent-a",
		Config:      map[string]string{"base_url": "https://telegram-api.test/"},
		Credentials: map[string]string{"bot_token": "telegram-token"},
	})
	if err != nil {
		t.Fatalf("配置 Telegram 失败: %v", err)
	}
	telegram, ok := router.GetForOwner("owner-a", ChannelTypeTelegram).(*channeladapters.TelegramChannel)
	if !ok || telegram.BaseURL() != "https://telegram-api.test" {
		t.Fatalf("Telegram 运行时配置未生效: channel=%+v ok=%v", telegram, ok)
	}

	_, err = service.UpsertChannelConfig(ctx, "owner-a", ChannelTypeDiscord, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"application_id": "discord-app",
			"base_url":       "https://discord-api.test/",
		},
		Credentials: map[string]string{"bot_token": "discord-token"},
	})
	if err != nil {
		t.Fatalf("配置 Discord 失败: %v", err)
	}
	discord, ok := router.GetForOwner("owner-a", ChannelTypeDiscord).(*channeladapters.DiscordChannel)
	if !ok || discord.BaseURL() != "https://discord-api.test" {
		t.Fatalf("Discord 运行时配置未生效: channel=%+v ok=%v", discord, ok)
	}
}

func TestControlServiceIncludesImplementedChannelsInSummaryCounts(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'telegram', 'agent-a', 'configured', '{}');
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'feishu', 'agent-a', 'connected', '{}');
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'weixin-personal', 'agent-a', 'configured', '{}');
INSERT INTO im_channel_accounts (owner_user_id, channel_type, account_id, user_id, status, config_json)
VALUES ('owner-a', 'weixin-personal', 'wx-account-a', 'wx-user-a', 'connected', '{}');
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
	if configured != 3 {
		t.Fatalf("已实现渠道应计入已配置渠道数，实际 %d", configured)
	}

	connected, err := service.CountConnectedChannels(context.Background(), "owner-a")
	if err != nil {
		t.Fatalf("统计已连接渠道失败: %v", err)
	}
	if connected != 2 {
		t.Fatalf("只有运行态 connected 或含已连接账号的渠道应计入已连接渠道数，实际 %d", connected)
	}

	activePairings, err := service.CountActivePairings(context.Background(), "owner-a")
	if err != nil {
		t.Fatalf("统计活跃配对失败: %v", err)
	}
	if activePairings != 1 {
		t.Fatalf("已实现渠道应计入活跃配对数，实际 %d", activePairings)
	}
}

func TestDeleteChannelConfigReportsCommittedWhenRuntimeStopFails(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, router)
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "token"},
		},
	); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	stopErr := errors.New("runtime stop failed")
	router.RegisterForOwner("owner-a", &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		stopErr:     stopErr,
	})

	err := service.DeleteChannelConfig(context.Background(), "owner-a", ChannelTypeTelegram)
	if !errors.Is(err, stopErr) {
		t.Fatalf("delete error = %v", err)
	}
	if effect, ok := ChannelControlMutationEffect(err); !ok || effect != ControlMutationCommitted {
		t.Fatalf("delete effect = %q ok=%v", effect, ok)
	}
	row, readErr := service.getChannelConfigRow(context.Background(), "owner-a", ChannelTypeTelegram)
	if readErr != nil || row != nil {
		t.Fatalf("committed delete must remain durable: row=%+v err=%v", row, readErr)
	}
}

type deletedSessionResolver struct{ deleted map[string]bool }

func (r deletedSessionResolver) ResolveDeliverySession(_ context.Context, _ string) (*protocol.Session, error) {
	return nil, nil
}

func (r deletedSessionResolver) IsSessionDeleted(_ context.Context, key string) (bool, error) {
	return r.deleted[key], nil
}

func TestControlServiceRotatesDeletedWildcardThreadSession(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	cfg := config.Config{DatabaseDriver: "sqlite"}
	router := NewRouter(cfg, db, nil, nil)
	router.SetSessionProjectionResolver(imTestSessions{})
	service := NewControlService(cfg, db, nil, router)
	if _, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc-group-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	}); err != nil {
		t.Fatalf("创建群级配对失败: %v", err)
	}
	request := IngressRequest{OwnerUserID: "owner-a", Channel: ChannelTypeFeishu, AccountID: "cli-a", ChatType: "group", Ref: "oc-group-a", ThreadID: "omt-thread-a"}
	_, firstKey, err := service.ResolveIngressSession(context.Background(), request)
	if err != nil {
		t.Fatalf("解析通配群话题 session 失败: %v", err)
	}
	if _, err = db.Exec("UPDATE im_pairing_sessions SET session_materialized = 1 WHERE session_key = ?", firstKey); err != nil {
		t.Fatalf("准备群话题 session 失败: %v", err)
	}
	_, secondKey, err := service.ResolveIngressSession(context.Background(), request)
	if err != nil {
		t.Fatalf("删除后的群话题 session 未恢复: %v", err)
	}
	if firstKey == secondKey || protocol.ParseSessionKey(secondKey).Generation == "" {
		t.Fatalf("群话题应生成新的 session 代次: first=%q second=%q", firstKey, secondKey)
	}
	parsed := protocol.ParseSessionKey(secondKey)
	if parsed.AccountID != "cli-a" || parsed.Ref != "oc-group-a" || parsed.ThreadID != "omt-thread-a" {
		t.Fatalf("群话题新 session 不应改变平台路由: %q parsed=%+v", secondKey, parsed)
	}
}

func TestControlServiceThreadedGroupIngressCreatesGroupScopedPendingPairing(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	var nextID int
	service.idFactory = func(prefix string) string {
		nextID++
		return fmt.Sprintf("%s-%d", prefix, nextID)
	}

	_, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID:  "owner-a",
		Channel:      ChannelTypeDiscord,
		ChatType:     "group",
		Ref:          "guild-1:channel-1",
		ThreadID:     "thread-a",
		ExternalName: "Release room",
		AgentID:      "agent-a",
	})
	var firstApproval *pairingApprovalError
	if !errors.As(err, &firstApproval) || firstApproval.PairingID != "pair-1" {
		t.Fatalf("首次群子线程入站应返回群级 pending pairing id: err=%v approval=%+v", err, firstApproval)
	}

	_, err = service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID:  "owner-a",
		Channel:      ChannelTypeDiscord,
		ChatType:     "group",
		Ref:          "guild-1:channel-1",
		ThreadID:     "thread-b",
		ExternalName: "Release room renamed",
		AgentID:      "agent-b",
	})
	var secondApproval *pairingApprovalError
	if !errors.As(err, &secondApproval) || secondApproval.PairingID != firstApproval.PairingID {
		t.Fatalf("同群不同子线程应复用群级 pending pairing: first=%+v second=%+v err=%v", firstApproval, secondApproval, err)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeDiscord,
		Status:      PairingStatusPending,
	})
	if err != nil {
		t.Fatalf("查询 pending 配对失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].PairingID != firstApproval.PairingID ||
		items[0].ThreadID != "" ||
		items[0].ExternalName != "Release room renamed" ||
		items[0].AgentID != "agent-b" {
		t.Fatalf("群子线程 pending 配对应归并到群级目标: %+v", items)
	}
}
