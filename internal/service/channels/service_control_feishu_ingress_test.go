package channels

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

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

func TestControlServicePrepareFeishuIngressAllowsChallengeWithoutStoredToken(t *testing.T) {
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
			"app_secret": "secret-a",
		},
	})
	if err != nil {
		t.Fatalf("配置飞书渠道失败: %v", err)
	}

	body := []byte(`{
		"type": "url_verification",
		"token": "feishu-side-token",
		"challenge": "challenge-token"
	}`)
	prepared, err := service.PrepareFeishuIngress(context.Background(), body, http.Header{})
	if err != nil {
		t.Fatalf("未保存 verification token 时也应允许飞书 URL 校验 challenge: %v", err)
	}
	if string(prepared.Body) != string(body) {
		t.Fatalf("URL 校验 body 不应被改写: %+v", prepared)
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
