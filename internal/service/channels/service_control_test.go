package channels

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
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
	if !wechat.SupportsGroup {
		t.Fatal("企业微信智能机器人通道应标记群聊能力")
	}
	weixinPersonal, ok := channelCatalogByType(ChannelTypeWeixinPersonal)
	if !ok {
		t.Fatal("缺少个人微信通道")
	}
	if weixinPersonal.RuntimeStatus != "ready" {
		t.Fatalf("个人微信应标记为内置可用，实际: %s", weixinPersonal.RuntimeStatus)
	}
	if weixinPersonal.Title != "微信" {
		t.Fatalf("微信通道前台标题不正确: %q", weixinPersonal.Title)
	}
	if !weixinPersonal.SupportsQRCode || weixinPersonal.SupportsGroup {
		t.Fatalf("个人微信能力标记不正确: %+v", weixinPersonal)
	}
	if !catalogHasCapability(weixinPersonal, channelmessage.CapabilityReceipt) {
		t.Fatalf("个人微信应声明本地消息回执能力: %+v", weixinPersonal.Capabilities)
	}
	feishu, ok := channelCatalogByType(ChannelTypeFeishu)
	if !ok {
		t.Fatal("缺少飞书通道")
	}
	for _, capability := range []channelmessage.Capability{
		channelmessage.CapabilityTyping,
		channelmessage.CapabilityThread,
		channelmessage.CapabilityReply,
		channelmessage.CapabilityReceipt,
	} {
		if !catalogHasCapability(feishu, capability) {
			t.Fatalf("飞书应声明 %s 能力: %+v", capability, feishu.Capabilities)
		}
	}
	telegram, ok := channelCatalogByType(ChannelTypeTelegram)
	if !ok {
		t.Fatal("缺少 Telegram 通道")
	}
	if !catalogHasCapability(telegram, channelmessage.CapabilityThread) ||
		!catalogHasCapability(telegram, channelmessage.CapabilityTyping) ||
		!catalogHasCapability(telegram, channelmessage.CapabilityReceipt) {
		t.Fatalf("Telegram 能力矩阵不完整: %+v", telegram.Capabilities)
	}
	if catalogHasCapability(wechat, channelmessage.CapabilityReceipt) {
		t.Fatalf("企业微信未返回稳定 message id，不应声明 receipt 能力: %+v", wechat.Capabilities)
	}
	dingtalk, ok := channelCatalogByType(ChannelTypeDingTalk)
	if !ok {
		t.Fatal("缺少钉钉通道")
	}
	if field, ok := catalogCredentialField(dingtalk, "robot_code"); !ok || field.Required {
		t.Fatalf("钉钉 Stream 回复不应强制要求 Robot Code: field=%+v ok=%v", field, ok)
	}
}

func catalogHasCapability(item ChannelCatalogItem, capability channelmessage.Capability) bool {
	for _, value := range item.Capabilities {
		if value == capability {
			return true
		}
	}
	return false
}

func catalogCredentialField(item ChannelCatalogItem, key string) (ChannelCredentialField, bool) {
	for _, field := range item.CredentialFields {
		if field.Key == key {
			return field, true
		}
	}
	return ChannelCredentialField{}, false
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

func TestControlServiceAllowsDingTalkStreamConfigWithoutRobotCode(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	item, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeDingTalk, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"client_id": "ding-client",
		},
		Credentials: map[string]string{
			"client_secret": "ding-secret",
		},
	})
	if err != nil {
		t.Fatalf("钉钉 Stream 配置不应强制要求 Robot Code: %v", err)
	}
	if item.ChannelType != ChannelTypeDingTalk || !item.Configured || !item.HasCredentials {
		t.Fatalf("钉钉 Stream 配置结果不正确: %+v", item)
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
	if latest.AccountID != "wx-account-1" || !strings.Contains(latest.Output, "微信已连接") {
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

func TestControlServiceCreatesManualPairingForKnownTarget(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType:  " telegram ",
		ChatType:     " group ",
		ExternalRef:  " -100123456 ",
		ThreadID:     " 42 ",
		ExternalName: " Release room ",
		AgentID:      " agent-a ",
	})
	if err != nil {
		t.Fatalf("手动创建 IM 配对失败: %v", err)
	}
	if created.PairingID == "" ||
		created.ChannelType != ChannelTypeTelegram ||
		created.ChatType != "group" ||
		created.ExternalRef != "-100123456" ||
		created.ThreadID != "42" ||
		created.ExternalName != "Release room" ||
		created.AgentID != "agent-a" ||
		created.Status != PairingStatusActive ||
		created.Source != PairingSourceManual {
		t.Fatalf("手动配对结果不正确: %+v", created)
	}

	agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeTelegram,
		ChatType:    "group",
		Ref:         "-100123456",
		ThreadID:    "42",
	})
	if err != nil {
		t.Fatalf("手动授权配对应允许入站路由: %v", err)
	}
	if agentID != "agent-a" {
		t.Fatalf("入站路由 agent 不正确: %q", agentID)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeTelegram,
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("查询手动配对失败: %v", err)
	}
	if len(items) != 1 || items[0].PairingID != created.PairingID || items[0].LastMessageAt == nil {
		t.Fatalf("手动配对列表结果不正确: %+v", items)
	}
}

func TestControlServiceAllowsManyExternalTargetsForOneAgent(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	targets := []string{"wx-user-1", "wx-user-2", "wx-user-3"}
	pairingIDs := map[string]bool{}
	sessionKeys := map[string]bool{}
	for _, ref := range targets {
		created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
			ChannelType:  ChannelTypeWeixinPersonal,
			ChatType:     "dm",
			ExternalRef:  ref,
			ExternalName: ref,
			AgentID:      "agent-a",
		})
		if err != nil {
			t.Fatalf("创建多用户 IM 配对失败 ref=%s err=%v", ref, err)
		}
		if created.AgentID != "agent-a" ||
			created.ChannelType != ChannelTypeWeixinPersonal ||
			created.ChatType != "dm" ||
			created.ExternalRef != ref ||
			created.Status != PairingStatusActive {
			t.Fatalf("多用户 IM 配对结果不正确 ref=%s item=%+v", ref, created)
		}
		if pairingIDs[created.PairingID] {
			t.Fatalf("不同外部用户不应复用 pairing id: %+v", created)
		}
		pairingIDs[created.PairingID] = true
		expectedSessionKey := "agent:agent-a:weixin-personal:dm:" + ref
		if created.SessionKey != expectedSessionKey {
			t.Fatalf("多用户 IM 配对应暴露稳定 session_key ref=%s got=%s want=%s", ref, created.SessionKey, expectedSessionKey)
		}
		if sessionKeys[created.SessionKey] {
			t.Fatalf("不同外部用户不应复用 session_key: %+v", created)
		}
		sessionKeys[created.SessionKey] = true

		agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
			OwnerUserID: "owner-a",
			Channel:     ChannelTypeWeixinPersonal,
			ChatType:    "dm",
			Ref:         ref,
		})
		if err != nil || agentID != "agent-a" {
			t.Fatalf("已授权外部用户应路由到同一 agent ref=%s agent=%q err=%v", ref, agentID, err)
		}
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeWeixinPersonal,
		Status:      PairingStatusActive,
		AgentID:     "agent-a",
	})
	if err != nil {
		t.Fatalf("查询多用户 IM 配对失败: %v", err)
	}
	if len(items) != len(targets) {
		t.Fatalf("同一 agent 应允许多个外部 IM 目标配对: %+v", items)
	}
	seenTargets := map[string]bool{}
	for _, item := range items {
		if item.AgentID != "agent-a" {
			t.Fatalf("多用户配对应保持同一 agent: %+v", item)
		}
		seenTargets[item.ExternalRef] = true
	}
	for _, ref := range targets {
		if !seenTargets[ref] {
			t.Fatalf("缺少外部用户配对 ref=%s items=%+v", ref, items)
		}
	}
}

func TestControlServiceCreatePairingUpdatesExistingTarget(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	first, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "dm",
		ExternalRef: "ou_user_1",
		AgentID:     "agent-a",
		Status:      PairingStatusPending,
	})
	if err != nil {
		t.Fatalf("创建初始配对失败: %v", err)
	}

	updated, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType:  ChannelTypeFeishu,
		ChatType:     "dm",
		ExternalRef:  "ou_user_1",
		ExternalName: "Alice",
		AgentID:      "agent-b",
		Status:       PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("重复创建同一目标应更新已有配对: %v", err)
	}
	if updated.PairingID != first.PairingID ||
		updated.ExternalName != "Alice" ||
		updated.AgentID != "agent-b" ||
		updated.Status != PairingStatusActive ||
		updated.Source != PairingSourceManual {
		t.Fatalf("重复创建配对应更新已有记录: first=%+v updated=%+v", first, updated)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{ChannelType: ChannelTypeFeishu})
	if err != nil {
		t.Fatalf("查询配对失败: %v", err)
	}
	if len(items) != 1 || items[0].PairingID != first.PairingID {
		t.Fatalf("重复创建不应产生多条配对: %+v", items)
	}
}

func TestControlServiceCreatesSeparatePendingPairingsForManyExternalTargets(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	var nextID int
	service.idFactory = func(prefix string) string {
		nextID++
		return fmt.Sprintf("%s-%d", prefix, nextID)
	}

	targets := []string{"wx-user-1", "wx-user-2"}
	for index, ref := range targets {
		_, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
			OwnerUserID:  "owner-a",
			Channel:      ChannelTypeWeixinPersonal,
			ChatType:     "dm",
			Ref:          ref,
			ExternalName: ref,
			AgentID:      "agent-a",
		})
		wantPairingID := fmt.Sprintf("pair-%d", index+1)
		var approval *pairingApprovalError
		if !errors.As(err, &approval) || approval.PairingID != wantPairingID {
			t.Fatalf("新外部用户应各自生成 pending pairing ref=%s err=%v approval=%+v want=%s", ref, err, approval, wantPairingID)
		}
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeWeixinPersonal,
		Status:      PairingStatusPending,
		AgentID:     "agent-a",
	})
	if err != nil {
		t.Fatalf("查询 pending IM 配对失败: %v", err)
	}
	if len(items) != len(targets) {
		t.Fatalf("不同外部用户的 pending 配对不应互相覆盖: %+v", items)
	}
	seenTargets := map[string]bool{}
	for _, item := range items {
		seenTargets[item.ExternalRef] = true
	}
	for _, ref := range targets {
		if !seenTargets[ref] {
			t.Fatalf("缺少 pending 外部用户配对 ref=%s items=%+v", ref, items)
		}
	}
}

func TestControlServiceResolveIngressAgentReturnsExistingPendingPairingID(t *testing.T) {
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
		Channel:      ChannelTypeTelegram,
		ChatType:     "group",
		Ref:          "-100123456",
		ThreadID:     "42",
		ExternalName: "Release room",
		AgentID:      "agent-a",
	})
	var firstApproval *pairingApprovalError
	if !errors.As(err, &firstApproval) || firstApproval.PairingID != "pair-1" {
		t.Fatalf("首次入站应返回真实 pending pairing id: err=%v approval=%+v", err, firstApproval)
	}

	_, err = service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID:  "owner-a",
		Channel:      ChannelTypeTelegram,
		ChatType:     "group",
		Ref:          "-100123456",
		ThreadID:     "42",
		ExternalName: "Release room renamed",
		AgentID:      "agent-b",
	})
	var secondApproval *pairingApprovalError
	if !errors.As(err, &secondApproval) || secondApproval.PairingID != firstApproval.PairingID {
		t.Fatalf("重复入站应返回已有 pending pairing id: first=%+v second=%+v err=%v", firstApproval, secondApproval, err)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeTelegram,
		Status:      PairingStatusPending,
	})
	if err != nil {
		t.Fatalf("查询 pending 配对失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].PairingID != firstApproval.PairingID ||
		items[0].ExternalName != "Release room renamed" ||
		items[0].AgentID != "agent-b" {
		t.Fatalf("重复入站应更新同一 pending 配对: %+v", items)
	}

	_, err = service.UpdatePairing(context.Background(), "owner-a", firstApproval.PairingID, UpdatePairingRequest{
		Status: ptrString(PairingStatusActive),
	})
	if err != nil {
		t.Fatalf("批准 pending 配对失败: %v", err)
	}
	agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeTelegram,
		ChatType:    "group",
		Ref:         "-100123456",
		ThreadID:    "42",
	})
	if err != nil || agentID != "agent-b" {
		t.Fatalf("批准后入站应路由到更新后的 agent: agent=%q err=%v", agentID, err)
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
