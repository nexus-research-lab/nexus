package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

func TestControlServiceReturnsHotReloadFailureWithoutPersistingBrokenConfig(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	candidate := &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		startErr:    fmt.Errorf("telegram runtime start failed"),
	}
	previous := routerChannelConfigurers[ChannelTypeTelegram]
	routerChannelConfigurers[ChannelTypeTelegram] = func(
		service *ControlService,
		ctx context.Context,
		cfg routerChannelConfiguration,
	) error {
		return service.registerConfiguredChannel(ctx, cfg, candidate)
	}
	t.Cleanup(func() {
		routerChannelConfigurers[ChannelTypeTelegram] = previous
	})

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, router)
	_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeTelegram, UpsertChannelConfigRequest{
		AgentID:     "agent-a",
		Credentials: map[string]string{"bot_token": "telegram-token"},
	})
	if err == nil || !strings.Contains(err.Error(), "telegram runtime start failed") {
		t.Fatalf("runtime 启动失败必须返回调用者: %v", err)
	}
	if effect, ok := ChannelControlMutationEffect(err); !ok || effect != ControlMutationNotApplied {
		t.Fatalf("已成功补偿的热重载失败 effect = %q ok=%v", effect, ok)
	}
	row, rowErr := service.getChannelConfigRow(context.Background(), "owner-a", ChannelTypeTelegram)
	if rowErr != nil {
		t.Fatalf("读取写后配置失败: %v", rowErr)
	}
	if row != nil {
		t.Fatalf("首次候选启动失败不得留下不可运行配置: %+v", row)
	}
	if router.GetForOwner("owner-a", ChannelTypeTelegram) != nil {
		t.Fatal("启动失败候选不应发布到 Router")
	}
	version, versionErr := service.GetChannelControlVersion(context.Background(), "owner-a")
	if versionErr != nil || version != 3 {
		t.Fatalf("失败候选应以新单调版本发布空快照: version=%d err=%v", version, versionErr)
	}
	if _, err = service.UpsertChannelConfigAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "stale-plan-token"},
		},
		1,
	); !errors.Is(err, ErrChannelControlVersionConflict) {
		t.Fatalf("热重载失败前的旧 plan 不得在回滚后重新命中: %v", err)
	}
}

func TestControlServiceFailedReplacementKeepsLastKnownGoodConfigAndRuntime(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	good := &recordingDeliveryChannel{channelType: ChannelTypeTelegram}
	failing := &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		startErr:    fmt.Errorf("replacement start failed"),
	}
	current := DeliveryChannel(good)
	previous := routerChannelConfigurers[ChannelTypeTelegram]
	routerChannelConfigurers[ChannelTypeTelegram] = func(
		service *ControlService,
		ctx context.Context,
		cfg routerChannelConfiguration,
	) error {
		return service.registerConfiguredChannel(ctx, cfg, current)
	}
	t.Cleanup(func() {
		routerChannelConfigurers[ChannelTypeTelegram] = previous
	})

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
			Config:      map[string]string{"base_url": "https://known-good.example"},
			Credentials: map[string]string{"bot_token": "known-good-token"},
		},
	); err != nil {
		t.Fatalf("建立已知可运行配置: %v", err)
	}
	current = failing
	if _, err := service.UpsertChannelConfigAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Config:      map[string]string{"base_url": "https://broken.example"},
			Credentials: map[string]string{"bot_token": "broken-token"},
		},
		2,
	); err == nil || !strings.Contains(err.Error(), "上一份可运行配置已保留") {
		t.Fatalf("失败替换结果 = %v", err)
	} else if effect, ok := ChannelControlMutationEffect(err); !ok || effect != ControlMutationNotApplied {
		t.Fatalf("失败替换 effect = %q ok=%v", effect, ok)
	}
	if got := router.GetForOwner("owner-a", ChannelTypeTelegram); got != good {
		t.Fatalf("失败候选替换了已知可运行 runtime: got=%T %p want=%p", got, got, good)
	}
	row, err := service.getChannelConfigRow(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
	)
	if err != nil {
		t.Fatal(err)
	}
	configValues, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		t.Fatal(err)
	}
	secretValues, err := service.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if configValues["base_url"] != "https://known-good.example" ||
		secretValues["bot_token"] != "known-good-token" ||
		row.Status != ChannelConfigStatusConnected {
		t.Fatalf("失败替换污染持久配置: row=%+v config=%v secret=%v", row, configValues, secretValues)
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 4 {
		t.Fatalf("失败替换应以新单调版本发布已知可运行快照: version=%d err=%v", version, err)
	}
	if _, err = service.UpsertChannelConfigAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "stale-plan-token"},
		},
		2,
	); !errors.Is(err, ErrChannelControlVersionConflict) {
		t.Fatalf("失败替换前的旧 plan 不得在回滚后重新命中: %v", err)
	}
}

func TestControlServiceRejectsStaleSecretRotationByPersistentVersion(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 1 {
		t.Fatalf("初始 Channel version = %d err=%v", version, err)
	}
	if _, err = service.UpsertChannelConfigAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "token-plan"},
		},
		version,
	); err != nil {
		t.Fatalf("初次带版本配置失败: %v", err)
	}
	staleVersion, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || staleVersion != 2 {
		t.Fatalf("初次配置后 version = %d err=%v", staleVersion, err)
	}

	if _, err = service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "token-newer-http"},
		},
	); err != nil {
		t.Fatalf("模拟后续 HTTP 凭据轮换失败: %v", err)
	}
	if _, err = service.UpsertChannelConfigAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeTelegram,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "token-stale-plan"},
		},
		staleVersion,
	); !errors.Is(err, ErrChannelControlVersionConflict) {
		t.Fatalf("旧 plan 必须被持久版本 CAS 拒绝: %v", err)
	}

	row, err := service.getChannelConfigRow(context.Background(), "owner-a", ChannelTypeTelegram)
	if err != nil {
		t.Fatal(err)
	}
	secrets, err := service.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if secrets["bot_token"] != "token-newer-http" {
		t.Fatalf("旧 plan 不得覆盖较新的凭据，实际 token=%q", secrets["bot_token"])
	}
	version, err = service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 3 {
		t.Fatalf("失败 CAS 不应推进 version: version=%d err=%v", version, err)
	}
}

func TestControlServiceSerializesUpsertAndDeleteThroughRuntimeReload(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	release := make(chan struct{})
	candidate := &blockingDeliveryChannel{
		recordingDeliveryChannel: recordingDeliveryChannel{channelType: ChannelTypeTelegram},
		startEntered:             make(chan struct{}),
		startRelease:             release,
	}
	previous := routerChannelConfigurers[ChannelTypeTelegram]
	routerChannelConfigurers[ChannelTypeTelegram] = func(
		service *ControlService,
		ctx context.Context,
		cfg routerChannelConfiguration,
	) error {
		return service.registerConfiguredChannel(ctx, cfg, candidate)
	}
	t.Cleanup(func() {
		routerChannelConfigurers[ChannelTypeTelegram] = previous
	})

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, router)
	upsertDone := make(chan error, 1)
	go func() {
		_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeTelegram, UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Credentials: map[string]string{"bot_token": "telegram-token"},
		})
		upsertDone <- err
	}()
	<-candidate.startEntered

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- service.DeleteChannelConfig(context.Background(), "owner-a", ChannelTypeTelegram)
	}()
	select {
	case err := <-deleteDone:
		t.Fatalf("delete 不得越过正在热重载的同一 owner+channel: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if err := <-upsertDone; err != nil {
		t.Fatalf("upsert 失败: %v", err)
	}
	if err := <-deleteDone; err != nil {
		t.Fatalf("delete 失败: %v", err)
	}
	exists, err := service.HasChannelConfig(context.Background(), "owner-a", ChannelTypeTelegram)
	if err != nil {
		t.Fatalf("核对配置删除失败: %v", err)
	}
	if exists || router.GetForOwner("owner-a", ChannelTypeTelegram) != nil {
		t.Fatalf("delete 完成后不得残留配置或刚发布的 runtime: exists=%v runtime=%T",
			exists,
			router.GetForOwner("owner-a", ChannelTypeTelegram),
		)
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

func TestControlServiceRejectsCatalogSecretsInPublicConfig(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	checked := 0
	for _, catalog := range channelCatalog() {
		for _, field := range catalog.CredentialFields {
			if !field.Secret {
				continue
			}
			checked++
			_, err := service.UpsertChannelConfig(
				context.Background(),
				"owner-a",
				catalog.ChannelType,
				UpsertChannelConfigRequest{
					AgentID: "agent-a",
					Config:  map[string]string{field.Key: "must-not-enter-config-json"},
				},
			)
			if err == nil || !strings.Contains(err.Error(), "must be supplied through the credentials channel") {
				t.Fatalf("%s.%s 进入普通 config JSON 应被拒绝: %v", catalog.ChannelType, field.Key, err)
			}
		}
	}
	if checked == 0 {
		t.Fatal("测试目录未包含 secret 字段")
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 1 {
		t.Fatalf("拒绝普通 JSON secret 不得推进版本: version=%d err=%v", version, err)
	}
}

func TestControlServiceFiltersCatalogSecretsFromDirtyPublicConfig(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	const dirtyConfig = `{
		"app_id":"cli_public",
		"base_url":"https://open.feishu.cn",
		"app_secret":"legacy-secret",
		"verification_token":"legacy-verification-token",
		"encrypt_key":"legacy-encrypt-key"
	}`
	if _, err := db.Exec(
		`INSERT INTO im_channel_configs
		 (owner_user_id, channel_type, agent_id, status, config_json)
		 VALUES (?, ?, ?, ?, ?)`,
		"owner-a",
		ChannelTypeFeishu,
		"agent-a",
		ChannelConfigStatusConfigured,
		dirtyConfig,
	); err != nil {
		t.Fatal(err)
	}

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	channels, err := service.ListChannels(context.Background(), "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	var view *ChannelConfigView
	for index := range channels {
		if channels[index].ChannelType == ChannelTypeFeishu {
			view = &channels[index]
			break
		}
	}
	if view == nil {
		t.Fatal("缺少飞书配置视图")
	}
	if view.PublicConfig["app_id"] != "cli_public" ||
		view.PublicConfig["base_url"] != "https://open.feishu.cn" {
		t.Fatalf("过滤 secret 不得删除公开字段: %+v", view.PublicConfig)
	}
	for _, key := range []string{"app_secret", "verification_token", "encrypt_key"} {
		if _, leaked := view.PublicConfig[key]; leaked {
			t.Fatalf("历史脏 config_json 泄露 catalog secret %s: %+v", key, view.PublicConfig)
		}
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
