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

type fakePersonalWeixinLoginClient struct {
	status channeladapters.PersonalWeixinQRStatusResponse
}

func (c *fakePersonalWeixinLoginClient) StartQRCode(context.Context, []string) (channeladapters.PersonalWeixinQRCodeResponse, error) {
	return channeladapters.PersonalWeixinQRCodeResponse{
		QRCode:             "qr-token-1",
		QRCodeImageContent: "weixin://qr-login",
	}, nil
}

func (c *fakePersonalWeixinLoginClient) PollQRCodeStatus(context.Context, string, string) (channeladapters.PersonalWeixinQRStatusResponse, error) {
	if strings.TrimSpace(c.status.Status) != "" {
		return c.status, nil
	}
	return channeladapters.PersonalWeixinQRStatusResponse{
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
	if configured == nil || !configured.HasCredentials || len(configured.Accounts) != 1 || configured.Accounts[0].AccountID != "wx-account-1" {
		t.Fatalf("登录后应保存 iLink 账号和 token: %+v", configured)
	}
	if configured.PublicConfig["account_id"] != "" || configured.PublicConfig["user_id"] != "" {
		t.Fatalf("个人微信账号不应再写回顶层 channel 配置: %+v", configured.PublicConfig)
	}
}

func TestControlServiceStoresMultipleWeixinPersonalLogins(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	var id int
	service.idFactory = func(prefix string) string {
		id++
		return fmt.Sprintf("%s-%d", prefix, id)
	}
	statuses := []channeladapters.PersonalWeixinQRStatusResponse{
		{
			Status:      "confirmed",
			BotToken:    "ilink-token-1",
			IlinkBotID:  "wx-account-1",
			IlinkUserID: "wx-user-1",
			BaseURL:     "https://ilink-a.test",
		},
		{
			Status:      "confirmed",
			BotToken:    "ilink-token-2",
			IlinkBotID:  "wx-account-2",
			IlinkUserID: "wx-user-2",
			BaseURL:     "https://ilink-b.test",
		},
	}
	var loginIndex int
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		status := statuses[loginIndex]
		loginIndex++
		return &fakePersonalWeixinLoginClient{status: status}
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

	for index := range statuses {
		started, err := service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
		if err != nil {
			t.Fatalf("启动第 %d 个个人微信扫码登录失败: %v", index+1, err)
		}
		waitChannelLoginStatus(t, service, "owner-a", ChannelTypeWeixinPersonal, started.LoginID, ChannelLoginStatusSucceeded)
	}

	accounts, err := service.listChannelAccountRows(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("读取个人微信账号失败: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("两个扫码微信账号应分别保存，实际: %+v", accounts)
	}
	seen := map[string]bool{}
	for _, account := range accounts {
		seen[account.AccountID] = true
	}
	if !seen["wx-account-1"] || !seen["wx-account-2"] {
		t.Fatalf("个人微信账号保存不完整: %+v", accounts)
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
	if configured == nil || !configured.HasCredentials || configured.PublicConfig["account_count"] != "2" || len(configured.Accounts) != 2 {
		t.Fatalf("个人微信频道应展示两个已登录账号: %+v", configured)
	}
	if configured.PublicConfig["account_id"] != "" || configured.PublicConfig["user_id"] != "" {
		t.Fatalf("个人微信多账号不应暴露最后扫码账号为顶层配置: %+v", configured.PublicConfig)
	}
}

func TestControlServiceMigratesLegacyWeixinPersonalAccountBeforeNewLogin(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	service.idFactory = func(prefix string) string {
		return prefix + "-legacy"
	}
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		return &fakePersonalWeixinLoginClient{status: channeladapters.PersonalWeixinQRStatusResponse{
			Status:      "confirmed",
			BotToken:    "new-token",
			IlinkBotID:  "wx-account-new",
			IlinkUserID: "wx-user-new",
			BaseURL:     "https://ilink-new.test",
		}}
	}
	_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeWeixinPersonal, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config: map[string]string{
			"base_url":   "https://ilink-legacy.test",
			"account_id": "wx-account-legacy",
			"user_id":    "wx-user-legacy",
		},
		Credentials: map[string]string{"ilink_bot_token": "legacy-token"},
	})
	if err != nil {
		t.Fatalf("准备旧个人微信配置失败: %v", err)
	}

	started, err := service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("启动新个人微信扫码登录失败: %v", err)
	}
	waitChannelLoginStatus(t, service, "owner-a", ChannelTypeWeixinPersonal, started.LoginID, ChannelLoginStatusSucceeded)

	accounts, err := service.listChannelAccountRows(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("读取个人微信账号失败: %v", err)
	}
	tokens := map[string]string{}
	for _, account := range accounts {
		secrets, decryptErr := service.decryptCredentials(account.CredentialsEncrypted)
		if decryptErr != nil {
			t.Fatalf("解密账号凭据失败 account=%s err=%v", account.AccountID, decryptErr)
		}
		tokens[account.AccountID] = secrets["ilink_bot_token"]
	}
	if tokens["wx-account-legacy"] != "legacy-token" || tokens["wx-account-new"] != "new-token" {
		t.Fatalf("旧账号应先迁移到账号表，新账号应独立保存: accounts=%+v tokens=%+v", accounts, tokens)
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
	if configured == nil || len(configured.Accounts) != 2 || configured.PublicConfig["account_id"] != "" || configured.PublicConfig["user_id"] != "" {
		t.Fatalf("个人微信账号视图不正确: %+v", configured)
	}
}

func TestControlServiceDeletesSingleWeixinPersonalAccount(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	var id int
	service.idFactory = func(prefix string) string {
		id++
		return fmt.Sprintf("%s-%d", prefix, id)
	}
	statuses := []channeladapters.PersonalWeixinQRStatusResponse{
		{Status: "confirmed", BotToken: "token-1", IlinkBotID: "wx-account-1", IlinkUserID: "wx-user-1"},
		{Status: "confirmed", BotToken: "token-2", IlinkBotID: "wx-account-2", IlinkUserID: "wx-user-2"},
	}
	var loginIndex int
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		status := statuses[loginIndex]
		loginIndex++
		return &fakePersonalWeixinLoginClient{status: status}
	}
	_, err := service.UpsertChannelConfig(context.Background(), "owner-a", ChannelTypeWeixinPersonal, UpsertChannelConfigRequest{
		AgentID: "agent-a",
		Config:  map[string]string{"base_url": "https://ilink.test"},
	})
	if err != nil {
		t.Fatalf("配置个人微信通道失败: %v", err)
	}
	for range statuses {
		started, startErr := service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
		if startErr != nil {
			t.Fatalf("启动个人微信扫码登录失败: %v", startErr)
		}
		waitChannelLoginStatus(t, service, "owner-a", ChannelTypeWeixinPersonal, started.LoginID, ChannelLoginStatusSucceeded)
	}

	updated, err := service.DeleteChannelAccount(context.Background(), "owner-a", ChannelTypeWeixinPersonal, "wx-account-1")
	if err != nil {
		t.Fatalf("删除单个微信账号失败: %v", err)
	}
	if updated == nil || len(updated.Accounts) != 1 || updated.Accounts[0].AccountID != "wx-account-2" {
		t.Fatalf("删除后应只保留第二个微信账号: %+v", updated)
	}
	accounts, err := service.listChannelAccountRows(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("读取账号表失败: %v", err)
	}
	if len(accounts) != 1 || accounts[0].AccountID != "wx-account-2" {
		t.Fatalf("账号表删除结果不正确: %+v", accounts)
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
