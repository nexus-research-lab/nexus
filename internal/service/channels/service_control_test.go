package channels

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestChannelCatalogMarksOnlyUnavailableChannelsPlanned(t *testing.T) {
	for _, item := range channelCatalog() {
		if item.ChannelType == ChannelTypeFeishu {
			if item.RuntimeStatus == "planned" {
				t.Fatalf("飞书主动投递已上线，不应标记为未上线")
			}
			continue
		}
		if item.RuntimeStatus != "planned" {
			t.Fatalf("%s 应标记为未上线，实际 runtime_status=%s", item.ChannelType, item.RuntimeStatus)
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

func TestControlServiceResolveChannelOwnerByConfig(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'feishu', 'agent-a', 'connected', '{"app_id":"cli_owner_a"}');
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-b', 'feishu', 'agent-b', 'disabled', '{"app_id":"cli_owner_b"}');
`); err != nil {
		t.Fatalf("准备 IM 配置失败: %v", err)
	}

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	ownerUserID, err := service.ResolveChannelOwnerByConfig(context.Background(), ChannelTypeFeishu, "app_id", "cli_owner_a")
	if err != nil {
		t.Fatalf("解析 owner 失败: %v", err)
	}
	if ownerUserID != "owner-a" {
		t.Fatalf("owner 不正确: %q", ownerUserID)
	}

	disabledOwner, err := service.ResolveChannelOwnerByConfig(context.Background(), ChannelTypeFeishu, "app_id", "cli_owner_b")
	if err != nil {
		t.Fatalf("解析 disabled owner 失败: %v", err)
	}
	if disabledOwner != "" {
		t.Fatalf("disabled 配置不应参与 owner 解析: %q", disabledOwner)
	}
}

func TestControlServicePrepareFeishuIngressVerifiesTokenAndOwner(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeFeishu, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"app_id": "cli_a",
		},
		Credentials: map[string]string{
			"app_secret":         "secret-a",
			"verification_token": "verification-token",
		},
	})
	if err != nil {
		t.Fatalf("配置飞书渠道失败: %v", err)
	}

	body := []byte(`{
		"schema": "2.0",
		"header": {
			"event_id": "evt-1",
			"event_type": "im.message.receive_v1",
			"app_id": "cli_a",
			"token": "verification-token"
		},
		"event": {
			"message": {
				"message_id": "om_1",
				"chat_id": "oc_group_123",
				"chat_type": "group",
				"message_type": "text",
				"content": "{\"text\":\"检查今天的定时任务发送情况\"}"
			}
		}
	}`)
	prepared, err := service.PrepareFeishuIngress(context.Background(), body, http.Header{})
	if err != nil {
		t.Fatalf("飞书回调安全校验失败: %v", err)
	}
	if prepared.OwnerUserID != "owner-a" || prepared.AppID != "cli_a" || string(prepared.Body) != string(body) {
		t.Fatalf("飞书回调准备结果不正确: %+v", prepared)
	}

	badBody := []byte(strings.ReplaceAll(string(body), "verification-token", "wrong-token"))
	if _, err = service.PrepareFeishuIngress(context.Background(), badBody, http.Header{}); !errors.Is(err, ErrFeishuCallbackUnauthorized) {
		t.Fatalf("错误 verification token 应拒绝，实际: %v", err)
	}
}

func TestControlServicePrepareFeishuIngressDecryptsAndVerifiesSignature(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeFeishu, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"app_id": "cli_enc",
		},
		Credentials: map[string]string{
			"app_secret":         "secret-a",
			"verification_token": "verification-token",
			"encrypt_key":        "encrypt-key",
		},
	})
	if err != nil {
		t.Fatalf("配置飞书加密渠道失败: %v", err)
	}

	plain := []byte(`{
		"schema": "2.0",
		"header": {
			"event_id": "evt-1",
			"event_type": "im.message.receive_v1",
			"app_id": "cli_enc",
			"token": "verification-token"
		},
		"event": {
			"message": {
				"message_id": "om_1",
				"chat_id": "oc_group_123",
				"chat_type": "group",
				"message_type": "text",
				"content": "{\"text\":\"停止每日新闻定时任务\"}"
			}
		}
	}`)
	body := encryptFeishuCallbackForTest(t, "encrypt-key", plain)
	prepared, err := service.PrepareFeishuIngress(context.Background(), body, signedFeishuHeaderForTest(body, "encrypt-key"))
	if err != nil {
		t.Fatalf("飞书加密回调准备失败: %v", err)
	}
	if prepared.OwnerUserID != "owner-a" || prepared.AppID != "cli_enc" || string(prepared.Body) != string(plain) {
		t.Fatalf("飞书加密回调准备结果不正确: %+v body=%s", prepared, prepared.Body)
	}

	if _, err = service.PrepareFeishuIngress(context.Background(), body, http.Header{}); !errors.Is(err, ErrFeishuCallbackUnauthorized) {
		t.Fatalf("缺少签名的加密回调应拒绝，实际: %v", err)
	}
}

func testChannelCredentialKey() string {
	return "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
}
