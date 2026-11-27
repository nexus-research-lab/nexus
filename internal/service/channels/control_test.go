// control_test.go 覆盖渠道配置控制、变更通知、失败处理、配对管理、Agent 删除与个人微信状态存储测试。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

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

func TestControlServiceReloadRollbackPreservesPersonalWeixinCursor(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	ctx := context.Background()
	if _, err := service.UpsertChannelConfig(ctx, "owner-a", ChannelTypeWeixinPersonal, UpsertChannelConfigRequest{
		AgentID: "agent-a",
	}); err != nil {
		t.Fatalf("建立个人微信配置失败: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, status, config_json, sync_cursor
) VALUES (?, ?, ?, ?, '{}', ?)`,
		"owner-a",
		ChannelTypeWeixinPersonal,
		"wx-account-a",
		ChannelConfigStatusConnected,
		"cursor-before-reload",
	); err != nil {
		t.Fatalf("建立个人微信账号失败: %v", err)
	}
	snapshot, err := service.captureChannelReloadSnapshot(ctx, "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("捕获热重载快照失败: %v", err)
	}
	failedVersion, err := service.withChannelControlMutation(ctx, "owner-a", snapshot.version, func(tx *sql.Tx) error {
		_, deleteErr := service.deleteChannelAccountRowWith(
			ctx,
			tx,
			"owner-a",
			ChannelTypeWeixinPersonal,
			"wx-account-a",
		)
		return deleteErr
	})
	if err != nil {
		t.Fatalf("模拟热重载前写入失败: %v", err)
	}
	if err = service.restoreChannelReloadSnapshot(ctx, snapshot, failedVersion); err != nil {
		t.Fatalf("恢复热重载快照失败: %v", err)
	}
	accounts, err := service.listChannelAccountRows(ctx, "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("读取恢复后的微信账号失败: %v", err)
	}
	if len(accounts) != 1 || accounts[0].SyncCursor != "cursor-before-reload" {
		t.Fatalf("热重载回滚丢失个人微信游标: %+v", accounts)
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

func TestControlServiceUpsertCancelsActiveQRLoginBeforeRebind(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	active := activeChannelLoginTestSession(
		"owner-a",
		ChannelTypeFeishu,
		"login-stale-qr",
		"",
	)
	active.view.QRPayload = "https://platform.test/stale-qr"
	active.view.QRPayloadType = "text"
	store := service.effectiveChannelLoginStore()
	store.sessions[active.view.LoginID] = active
	store.active[active.activeKey] = active.view.LoginID

	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeFeishu,
		UpsertChannelConfigRequest{
			AgentID:     "agent-b",
			Config:      map[string]string{"app_id": "cli-new"},
			Credentials: map[string]string{"app_secret": "secret-new"},
		},
	); err != nil {
		t.Fatalf("重绑 Channel Agent 不应失败: %v", err)
	}
	view := active.snapshot()
	if view.Status != ChannelLoginStatusCancelled || view.QRPayload != "" || view.QRPayloadType != "" {
		t.Fatalf("Channel 重绑后旧扫码会话仍可见: %+v", view)
	}
	store.mu.Lock()
	_, activeStillIndexed := store.active[active.activeKey]
	store.mu.Unlock()
	if activeStillIndexed {
		t.Fatal("Channel 重绑后不得保留旧扫码会话的 active 索引")
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

func TestControlServiceNotifiesOnlyCommittedControlChanges(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	var owners []string
	service.SetControlChangeNotifier(func(_ context.Context, ownerUserID string) {
		owners = append(owners, ownerUserID)
	})
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeTelegram,
		ChatType:    "dm",
		ExternalRef: "chat-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	if len(owners) != 1 || owners[0] != "owner-a" {
		t.Fatalf("提交成功应恰好通知一次 owner: %v", owners)
	}

	// 版本冲突的写入没有提交，不应产生失效通知。
	if err = service.DeletePairingAtVersion(context.Background(), "owner-a", created.PairingID, 1); err == nil {
		t.Fatal("过期版本删除应失败")
	}
	if len(owners) != 1 {
		t.Fatalf("未提交的写入不应通知: %v", owners)
	}

	if err = service.touchPairing(context.Background(), "owner-a", created.PairingID); err != nil {
		t.Fatalf("刷新最近消息时间失败: %v", err)
	}
	if len(owners) != 2 {
		t.Fatalf("最近消息时间更新应通知配对目录: %v", owners)
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

func TestChannelControlTransactionFailureCarriesNotAppliedEvidence(t *testing.T) {
	db := newChannelTestDB(t)
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := service.withChannelControlMutation(
		context.Background(),
		"owner-a",
		0,
		nil,
	)
	if err == nil {
		t.Fatal("closed database mutation unexpectedly succeeded")
	}
	if effect, ok := ChannelControlMutationEffect(err); !ok || effect != ControlMutationNotApplied {
		t.Fatalf("begin failure effect = %q ok=%v", effect, ok)
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

func TestControlServiceUpdatePairingPatchesOnlyRequestedFields(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeTelegram,
		ChatType:    "dm",
		ExternalRef: "chat-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
		Source:      PairingSourceIngress,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 2 {
		t.Fatalf("创建 pairing 应推进 Channel version: version=%d err=%v", version, err)
	}
	const lastMessageAt = "2040-01-02 03:04:05"
	if _, err = db.Exec(
		"UPDATE im_pairings SET last_message_at = ? WHERE owner_user_id = ? AND pairing_id = ?",
		lastMessageAt,
		"owner-a",
		created.PairingID,
	); err != nil {
		t.Fatalf("准备 ingress 时间失败: %v", err)
	}

	name := "Renamed chat"
	status := PairingStatusPending
	updated, err := service.UpdatePairing(context.Background(), "owner-a", created.PairingID, UpdatePairingRequest{
		ExternalName: &name,
		Status:       &status,
	})
	if err != nil {
		t.Fatalf("字段 patch 失败: %v", err)
	}
	version, err = service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 3 {
		t.Fatalf("更新 pairing 应推进 Channel version: version=%d err=%v", version, err)
	}
	if updated.ExternalName != name || updated.Status != status || updated.Source != PairingSourceIngress {
		t.Fatalf("字段 patch 结果不正确: %+v", updated)
	}
	var storedLastMessageAt string
	var storedSource string
	if err = db.QueryRow(
		"SELECT CAST(last_message_at AS TEXT), source FROM im_pairings WHERE owner_user_id = ? AND pairing_id = ?",
		"owner-a",
		created.PairingID,
	).Scan(&storedLastMessageAt, &storedSource); err != nil {
		t.Fatalf("读取字段 patch 结果失败: %v", err)
	}
	if storedLastMessageAt != lastMessageAt || storedSource != PairingSourceIngress {
		t.Fatalf("人工更新不得覆盖 ingress 字段: last_message_at=%q source=%q", storedLastMessageAt, storedSource)
	}
}

func TestControlServiceGroupPairingRoutesThreadedIngress(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	groupPairing, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc_group_123",
		AgentID:     "agent-a",
	})
	if err != nil {
		t.Fatalf("创建群级 IM 配对失败: %v", err)
	}
	topicPairing, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc_group_123",
		ThreadID:    "topic-override",
		AgentID:     "agent-b",
	})
	if err != nil {
		t.Fatalf("创建话题级 IM 配对失败: %v", err)
	}

	agentID, err := service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		ChatType:    "group",
		Ref:         "oc_group_123",
		ThreadID:    "topic-1",
	})
	if err != nil || agentID != "agent-a" {
		t.Fatalf("群级配对应接住子线程入站: agent=%q err=%v", agentID, err)
	}
	agentID, err = service.ResolveIngressAgent(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeFeishu,
		ChatType:    "group",
		Ref:         "oc_group_123",
		ThreadID:    "topic-override",
	})
	if err != nil || agentID != "agent-b" {
		t.Fatalf("话题级配对应优先于群级配对: agent=%q err=%v", agentID, err)
	}

	items, err := service.ListPairings(context.Background(), "owner-a", PairingQuery{
		ChannelType: ChannelTypeFeishu,
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("查询 active 配对失败: %v", err)
	}
	seen := map[string]PairingView{}
	for _, item := range items {
		seen[item.PairingID] = item
	}
	if seen[groupPairing.PairingID].LastMessageAt == nil || seen[topicPairing.PairingID].LastMessageAt == nil {
		t.Fatalf("命中的配对应更新 last_message_at: %+v", items)
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

func TestControlServiceAgentDeletionReportsRuntimeStopFailureAfterCommit(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	seedAgentChannelImpact(t, db)

	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())
	channel := &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		stopErr:     errors.New("runtime stop failed"),
	}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", channel); err != nil {
		t.Fatal(err)
	}
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)

	err := service.CoordinateAgentDeletion(
		context.Background(),
		"owner-a",
		"agent-a",
		func(ctx context.Context) error {
			return deleteSeededAgentChannelImpact(ctx, db)
		},
	)
	if err == nil || !errors.Is(err, channel.stopErr) {
		t.Fatalf("持久删除成功但 runtime 停止失败必须返回 reconcile error: %v", err)
	}
	if router.GetForOwner("owner-a", ChannelTypeTelegram) != nil {
		t.Fatal("停止失败的 runtime 也必须从 Router 注销，避免继续接收新流量")
	}
	assertNoAgentChannelImpact(t, db)
}

func TestControlServiceSerializesAgentDeletionWithChannelMutation(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	seedAgentChannelImpact(t, db)
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)

	deleteEntered := make(chan struct{})
	deleteRelease := make(chan struct{})
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- service.CoordinateAgentDeletion(
			context.Background(),
			"owner-a",
			"agent-a",
			func(ctx context.Context) error {
				close(deleteEntered)
				<-deleteRelease
				return deleteSeededAgentChannelImpact(ctx, db)
			},
		)
	}()
	<-deleteEntered

	upsertDone := make(chan error, 1)
	go func() {
		_, err := service.UpsertChannelConfig(
			context.Background(),
			"owner-a",
			ChannelTypeTelegram,
			UpsertChannelConfigRequest{
				AgentID:     "agent-b",
				Credentials: map[string]string{"bot_token": "new-token"},
			},
		)
		upsertDone <- err
	}()
	select {
	case err := <-upsertDone:
		t.Fatalf("Channel mutation 不得越过 Agent 删除协调: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(deleteRelease)
	if err := <-deleteDone; err != nil {
		t.Fatalf("Agent 删除协调失败: %v", err)
	}
	if err := <-upsertDone; err != nil {
		t.Fatalf("删除完成后的新 Channel 写入失败: %v", err)
	}
	row, err := service.getChannelConfigRow(context.Background(), "owner-a", ChannelTypeTelegram)
	if err != nil || row == nil || row.AgentID != "agent-b" {
		t.Fatalf("串行化后的新配置不正确: row=%+v err=%v", row, err)
	}
}

func seedAgentChannelImpact(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO channel_control_versions (owner_user_id, version) VALUES ('owner-a', 1);
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES ('owner-a', 'telegram', 'agent-a', 'configured', '{}');
INSERT INTO im_channel_accounts (owner_user_id, channel_type, account_id, status, config_json)
VALUES ('owner-a', 'telegram', 'account-a', 'connected', '{}');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, agent_id, status, source
) VALUES ('pair-a', 'owner-a', 'telegram', 'dm', 'chat-a', 'agent-a', 'active', 'manual');`)
	if err != nil {
		t.Fatal(err)
	}
}

func deleteSeededAgentChannelImpact(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, query := range []string{
		"UPDATE channel_control_versions SET version = version + 1 WHERE owner_user_id = 'owner-a'",
		"DELETE FROM im_pairings WHERE owner_user_id = 'owner-a' AND agent_id = 'agent-a'",
		"DELETE FROM im_channel_accounts WHERE owner_user_id = 'owner-a' AND channel_type = 'telegram'",
		"DELETE FROM im_channel_configs WHERE owner_user_id = 'owner-a' AND agent_id = 'agent-a'",
	} {
		if _, err = tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func assertNoAgentChannelImpact(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"im_channel_configs", "im_channel_accounts", "im_pairings"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(1) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s 删除后仍有 %d 条记录", table, count)
		}
	}
}

func TestPersonalWeixinRuntimeStorePersistsCursorAndMarksExpiredLogin(t *testing.T) {
	db := newChannelTestDB(t)
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if _, err := db.Exec(`
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, status, config_json, sync_cursor
) VALUES ('owner-a', 'weixin-personal', 'account-1', 'connected', '{}', 'cursor-old')`); err != nil {
		t.Fatal(err)
	}

	cursor, err := service.LoadPersonalWeixinCursor(context.Background(), "owner-a", "account-1")
	if err != nil || cursor != "cursor-old" {
		t.Fatalf("读取个人微信游标失败: cursor=%q err=%v", cursor, err)
	}
	if err = service.SavePersonalWeixinCursor(context.Background(), "owner-a", "account-1", "cursor-new"); err != nil {
		t.Fatalf("保存个人微信游标失败: %v", err)
	}
	cursor, err = service.LoadPersonalWeixinCursor(context.Background(), "owner-a", "account-1")
	if err != nil || cursor != "cursor-new" {
		t.Fatalf("个人微信游标未持久化: cursor=%q err=%v", cursor, err)
	}

	if err = service.MarkPersonalWeixinLoginExpired(
		context.Background(),
		"owner-a",
		"account-1",
		"scan QR code to reconnect",
	); err != nil {
		t.Fatalf("标记个人微信登录失效失败: %v", err)
	}
	var status string
	var lastError string
	if err = db.QueryRow(`
SELECT status, last_error, sync_cursor
FROM im_channel_accounts
WHERE owner_user_id = 'owner-a' AND channel_type = 'weixin-personal' AND account_id = 'account-1'`,
	).Scan(&status, &lastError, &cursor); err != nil {
		t.Fatal(err)
	}
	if status != ChannelConfigStatusError || cursor != "" || !strings.Contains(lastError, "scan QR code") {
		t.Fatalf("个人微信登录失效投影不正确: status=%q cursor=%q last_error=%q", status, cursor, lastError)
	}
}
