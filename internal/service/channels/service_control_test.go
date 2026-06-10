package channels

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestChannelCatalogMarksImplementedChannelsReady(t *testing.T) {
	for _, item := range channelCatalog() {
		if item.RuntimeStatus == "planned" {
			t.Fatalf("%s 不应再标记为未上线", item.ChannelType)
		}
	}
	wechat, ok := channelCatalogByType(ChannelTypeWeChat)
	if !ok {
		t.Fatal("缺少企业微信通道")
	}
	if wechat.SupportsGroup {
		t.Fatal("企业微信自建应用通道不应标记群聊能力")
	}
	weixinPersonal, ok := channelCatalogByType(ChannelTypeWeixinPersonal)
	if !ok {
		t.Fatal("缺少个人微信通道")
	}
	if weixinPersonal.RuntimeStatus != "ready" {
		t.Fatalf("个人微信应标记为内置可用，实际: %s", weixinPersonal.RuntimeStatus)
	}
	if !weixinPersonal.SupportsQRCode || weixinPersonal.SupportsGroup {
		t.Fatalf("个人微信能力标记不正确: %+v", weixinPersonal)
	}
}

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
			credentials: map[string]string{"client_secret": "ding-secret"},
			want:        "robot_code is required",
		},
		{
			name:        "wechat",
			channelType: ChannelTypeWeChat,
			config:      map[string]string{"corp_id": "ww-corp"},
			want:        "agent_id is required",
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

func TestControlServiceConfiguresWeixinPersonalWithoutSecrets(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	item, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeWeixinPersonal, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"base_url": "https://ilink.test",
		},
	})
	if err != nil {
		t.Fatalf("配置个人微信通道失败: %v", err)
	}
	if item.ChannelType != ChannelTypeWeixinPersonal || item.RuntimeStatus != "ready" || !item.Configured {
		t.Fatalf("个人微信配置结果不正确: %+v", item)
	}
	if item.HasCredentials {
		t.Fatalf("个人微信配置阶段不应要求 Nexus 保存 iLink token: %+v", item)
	}
}

func TestControlServiceStartsWeixinPersonalLogin(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	service.idFactory = func(prefix string) string {
		return prefix + "-1"
	}
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		return &fakePersonalWeixinLoginClient{}
	}
	_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeWeixinPersonal, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"base_url": "https://ilink.test",
		},
	})
	if err != nil {
		t.Fatalf("配置个人微信通道失败: %v", err)
	}

	started, err := service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("启动个人微信扫码登录失败: %v", err)
	}
	if started.LoginID != "channel_login-1" || started.Status != ChannelLoginStatusRunning {
		t.Fatalf("初始登录状态不正确: %+v", started)
	}
	if started.QRPayload != "weixin://qr-login" {
		t.Fatalf("登录二维码不正确: %+v", started)
	}

	latest := waitChannelLoginStatus(t, service, "owner-a", ChannelTypeWeixinPersonal, started.LoginID, ChannelLoginStatusSucceeded)
	if latest.AccountID != "wx-account-1" || !strings.Contains(latest.Output, "个人微信已连接") {
		t.Fatalf("登录完成状态不正确: %+v", latest)
	}
	items, err := service.ListChannels(context.Background(), "owner-a")
	if err != nil {
		t.Fatalf("读取频道配置失败: %v", err)
	}
	var configured *ChannelConfigView
	for index := range items {
		if items[index].ChannelType == ChannelTypeWeixinPersonal {
			configured = &items[index]
			break
		}
	}
	if configured == nil || !configured.HasCredentials || configured.PublicConfig["account_id"] != "wx-account-1" {
		t.Fatalf("登录后应保存 iLink 账号和 token: %+v", configured)
	}
}

func TestControlServiceRejectsUnsupportedChannelLogin(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	_, err := service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeWeChat)
	if !errors.Is(err, ErrChannelLoginUnsupported) {
		t.Fatalf("企业微信不应走个人微信 iLink 扫码登录，实际: %v", err)
	}
}

func TestControlServiceIncludesImplementedChannelsInSummaryCounts(t *testing.T) {
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
	if configured != 1 {
		t.Fatalf("已实现渠道应计入已配置渠道数，实际 %d", configured)
	}

	activePairings, err := service.CountActivePairings(context.Background(), "owner-a")
	if err != nil {
		t.Fatalf("统计活跃配对失败: %v", err)
	}
	if activePairings != 1 {
		t.Fatalf("已实现渠道应计入活跃配对数，实际 %d", activePairings)
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

func TestControlServicePrepareWeChatIngressDecryptsAndVerifiesSignature(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	encodingAESKey := testWeChatEncodingAESKey()
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeWeChat, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"corp_id":  "ww_corp",
			"agent_id": "100001",
		},
		Credentials: map[string]string{
			"corp_secret":      "corp-secret",
			"token":            "wechat-token",
			"encoding_aes_key": encodingAESKey,
		},
	})
	if err != nil {
		t.Fatalf("配置企业微信渠道失败: %v", err)
	}

	plain := []byte(`<xml>
		<ToUserName><![CDATA[ww_corp]]></ToUserName>
		<FromUserName><![CDATA[zhangsan]]></FromUserName>
		<CreateTime>1700000000</CreateTime>
		<MsgType><![CDATA[text]]></MsgType>
		<Content><![CDATA[检查本周日报任务]]></Content>
		<MsgId>msg-1</MsgId>
		<AgentID>100001</AgentID>
	</xml>`)
	encrypted := encryptWeChatCallbackForTest(t, encodingAESKey, plain, "ww_corp")
	signature := weChatCallbackSignature("wechat-token", "1700000001", "nonce-1", encrypted)
	body := []byte(fmt.Sprintf(`<xml>
		<ToUserName><![CDATA[ww_corp]]></ToUserName>
		<AgentID><![CDATA[100001]]></AgentID>
		<Encrypt><![CDATA[%s]]></Encrypt>
	</xml>`, encrypted))
	request, err := http.NewRequest(
		http.MethodPost,
		"/nexus/v1/channels/wechat/messages?msg_signature="+signature+"&timestamp=1700000001&nonce=nonce-1",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("构造企业微信请求失败: %v", err)
	}

	prepared, err := service.PrepareWeChatIngress(context.Background(), body, request)
	if err != nil {
		t.Fatalf("企业微信回调准备失败: %v", err)
	}
	if prepared.OwnerUserID != "owner-a" || prepared.CorpID != "ww_corp" || string(prepared.Body) != string(plain) {
		t.Fatalf("企业微信回调准备结果不正确: %+v body=%s", prepared, prepared.Body)
	}
	ingressRequest, ignored, err := DecodeWeChatIngressCallback(prepared.Body)
	if err != nil {
		t.Fatalf("企业微信回调解析失败: %v", err)
	}
	if ignored != "" || ingressRequest == nil {
		t.Fatalf("企业微信文本消息不应被忽略: request=%+v ignored=%s", ingressRequest, ignored)
	}
	if ingressRequest.Ref != "zhangsan" || ingressRequest.Content != "检查本周日报任务" {
		t.Fatalf("企业微信入口请求不正确: %+v", ingressRequest)
	}

	badRequest, err := http.NewRequest(
		http.MethodPost,
		"/nexus/v1/channels/wechat/messages?msg_signature=bad&timestamp=1700000001&nonce=nonce-1",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("构造错误企业微信请求失败: %v", err)
	}
	if _, err = service.PrepareWeChatIngress(context.Background(), body, badRequest); !errors.Is(err, ErrWeChatCallbackUnauthorized) {
		t.Fatalf("错误签名的企业微信回调应拒绝，实际: %v", err)
	}
}

type fakePersonalWeixinLoginClient struct{}

func (c *fakePersonalWeixinLoginClient) StartQRCode(context.Context, []string) (weixinQRCodeResponse, error) {
	return weixinQRCodeResponse{
		QRCode:             "qr-token-1",
		QRCodeImageContent: "weixin://qr-login",
	}, nil
}

func (c *fakePersonalWeixinLoginClient) PollQRCodeStatus(context.Context, string, string) (weixinQRStatusResponse, error) {
	return weixinQRStatusResponse{
		Status:      "confirmed",
		BotToken:    "ilink-token-1",
		IlinkBotID:  "wx-account-1",
		IlinkUserID: "wx-user-1",
		BaseURL:     "https://ilink.test",
	}, nil
}

func waitChannelLoginStatus(
	t *testing.T,
	service *ControlService,
	ownerUserID string,
	channelType string,
	loginID string,
	status string,
) *ChannelLoginView {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		view, err := service.GetChannelLogin(context.Background(), ownerUserID, channelType, loginID)
		if err != nil {
			t.Fatalf("读取登录状态失败: %v", err)
		}
		if view.Status == status {
			return view
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, err := service.GetChannelLogin(context.Background(), ownerUserID, channelType, loginID)
	if err != nil {
		t.Fatalf("读取最终登录状态失败: %v", err)
	}
	t.Fatalf("等待登录状态超时: got=%s want=%s view=%+v", view.Status, status, view)
	return nil
}

func testChannelCredentialKey() string {
	return "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
}

func testWeChatEncodingAESKey() string {
	return strings.TrimRight(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), "=")
}

func encryptWeChatCallbackForTest(t *testing.T, encodingAESKey string, plain []byte, receiveID string) string {
	t.Helper()
	key, err := decodeWeChatAESKey(encodingAESKey)
	if err != nil {
		t.Fatalf("测试企业微信 AES key 无效: %v", err)
	}
	packet := bytes.NewBufferString("0123456789abcdef")
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(plain)))
	packet.Write(length)
	packet.Write(plain)
	packet.WriteString(receiveID)
	padded := appendWeChatPKCS7PaddingForTest(packet.Bytes(), aes.BlockSize)

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("创建测试企业微信 cipher 失败: %v", err)
	}
	cipherText := append([]byte(nil), padded...)
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(cipherText, cipherText)
	return base64.StdEncoding.EncodeToString(cipherText)
}

func appendWeChatPKCS7PaddingForTest(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}
