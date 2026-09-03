package channels

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

type fakePersonalWeixinLoginClient struct {
	status   channeladapters.PersonalWeixinQRStatusResponse
	startErr error
}

type fakeChannelRegistrationClient struct {
	credentials map[string]string
	startErr    error
}

func (c *fakeChannelRegistrationClient) Start(context.Context) (*appregistration.StartResult, error) {
	if c.startErr != nil {
		return nil, c.startErr
	}
	return &appregistration.StartResult{
		DeviceCode:              "registration-device-code",
		VerificationURIComplete: "https://platform.test/scan",
		ExpiresIn:               60,
		Interval:                1,
	}, nil
}

func (c *fakeChannelRegistrationClient) Poll(context.Context, string) (*appregistration.PollResult, error) {
	return &appregistration.PollResult{
		Status:      appregistration.StatusSucceeded,
		Credentials: c.credentials,
	}, nil
}

func activeChannelLoginTestSession(
	ownerUserID string,
	channelType string,
	loginID string,
	authorizationBinding string,
) *channelLoginSession {
	now := time.Now()
	return &channelLoginSession{
		ownerUserID:          ownerUserID,
		channelType:          channelType,
		activeKey:            channelLoginActiveKey(ownerUserID, channelType),
		authorizationBinding: authorizationBinding,
		view: ChannelLoginView{
			LoginID:     loginID,
			ChannelType: channelType,
			Status:      ChannelLoginStatusRunning,
			StartedAt:   now,
			UpdatedAt:   now,
			ExpiresAt:   now.Add(time.Minute),
		},
	}
}

func TestGetCurrentChannelLoginOnlyReadsUniqueUnboundWebSession(t *testing.T) {
	store := newChannelLoginStore()
	current := activeChannelLoginTestSession(
		"owner-a",
		ChannelTypeWeixinPersonal,
		"login-web",
		"",
	)
	foreign := activeChannelLoginTestSession(
		"owner-b",
		ChannelTypeWeixinPersonal,
		"login-foreign",
		"",
	)
	store.sessions[current.view.LoginID] = current
	store.active[current.activeKey] = current.view.LoginID
	store.sessions[foreign.view.LoginID] = foreign
	store.active[foreign.activeKey] = foreign.view.LoginID

	service := &ControlService{
		loginStore: store,
		idFactory: func(string) string {
			t.Fatal("read-only reconciliation generated a login ID")
			return ""
		},
		weixinLoginClientFactory: func(string, map[string]string) personalWeixinLoginClient {
			t.Fatal("read-only reconciliation started a Weixin login")
			return nil
		},
		registrationClientFactory: func(string) appregistration.Client {
			t.Fatal("read-only reconciliation started platform registration")
			return nil
		},
	}
	view, err := service.GetCurrentChannelLogin(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatalf("read current Web login: %v", err)
	}
	if view.LoginID != "login-web" || view.ChannelType != ChannelTypeWeixinPersonal {
		t.Fatalf("current Web login = %+v", view)
	}
	if _, err = service.GetCurrentChannelLogin(
		context.Background(),
		"owner-c",
		ChannelTypeWeixinPersonal,
	); !errors.Is(err, ErrChannelLoginNotFound) {
		t.Fatalf("cross-owner lookup error = %v", err)
	}
}

func TestGetCurrentChannelLoginFailsClosedForAmbiguousOrBoundState(t *testing.T) {
	tests := []struct {
		name     string
		sessions []*channelLoginSession
		activeID string
	}{
		{
			name: "multiple active Web sessions",
			sessions: []*channelLoginSession{
				activeChannelLoginTestSession("owner-a", ChannelTypeFeishu, "login-1", ""),
				activeChannelLoginTestSession("owner-a", ChannelTypeFeishu, "login-2", ""),
			},
			activeID: "login-1",
		},
		{
			name: "conversational authorization binding",
			sessions: []*channelLoginSession{
				activeChannelLoginTestSession("owner-a", ChannelTypeFeishu, "login-bound", "authorization-1"),
			},
			activeID: "login-bound",
		},
		{
			name: "active index mismatch",
			sessions: []*channelLoginSession{
				activeChannelLoginTestSession("owner-a", ChannelTypeFeishu, "login-web", ""),
			},
			activeID: "different-login",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newChannelLoginStore()
			for _, session := range test.sessions {
				store.sessions[session.view.LoginID] = session
			}
			store.active[channelLoginActiveKey("owner-a", ChannelTypeFeishu)] = test.activeID
			service := &ControlService{loginStore: store}
			if _, err := service.GetCurrentChannelLogin(
				context.Background(),
				"owner-a",
				ChannelTypeFeishu,
			); !errors.Is(err, ErrChannelLoginState) {
				t.Fatalf("ambiguous current login error = %v", err)
			}
		})
	}
}

func TestGetCurrentChannelLoginAbsenceRemainsUnprovenToCaller(t *testing.T) {
	service := &ControlService{loginStore: newChannelLoginStore()}
	if _, err := service.GetCurrentChannelLogin(
		context.Background(),
		"owner-a",
		ChannelTypeFeishu,
	); !errors.Is(err, ErrChannelLoginNotFound) {
		t.Fatalf("missing current login error = %v", err)
	}
}

func (c *fakePersonalWeixinLoginClient) StartQRCode(context.Context, []string) (channeladapters.PersonalWeixinQRCodeResponse, error) {
	if c.startErr != nil {
		return channeladapters.PersonalWeixinQRCodeResponse{}, c.startErr
	}
	return channeladapters.PersonalWeixinQRCodeResponse{
		QRCode:             "qr-token-1",
		QRCodeImageContent: "weixin://qr-login",
	}, nil
}

func TestControlServiceMarksRemoteLoginStartFailureUnknown(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	_, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeFeishu,
		UpsertChannelConfigRequest{
			AgentID: "agent-a",
			Config:  map[string]string{"app_id": "app-id"},
			Credentials: map[string]string{
				"app_secret": "app-secret",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed Channel config: %v", err)
	}
	startErr := errors.New("remote response was lost")
	service.registrationClientFactory = func(string) appregistration.Client {
		return &fakeChannelRegistrationClient{startErr: startErr}
	}

	_, err = service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeFeishu)
	if !errors.Is(err, startErr) {
		t.Fatalf("start error = %v", err)
	}
	if effect, ok := ChannelControlMutationEffect(err); !ok || effect != ControlMutationUnknown {
		t.Fatalf("remote start effect = %q ok=%v", effect, ok)
	}
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

func TestControlServiceRegistersOfficialQRCodeChannels(t *testing.T) {
	cases := []struct {
		channelType string
		credentials map[string]string
		publicKey   string
		publicValue string
	}{
		{
			channelType: ChannelTypeFeishu,
			credentials: map[string]string{"client_id": "cli_feishu", "client_secret": "feishu-secret"},
			publicKey:   "app_id",
			publicValue: "cli_feishu",
		},
		{
			channelType: ChannelTypeDingTalk,
			credentials: map[string]string{"client_id": "ding-client", "client_secret": "ding-secret"},
			publicKey:   "client_id",
			publicValue: "ding-client",
		},
		{
			channelType: ChannelTypeWeChat,
			credentials: map[string]string{"bot_id": "wecom-bot", "secret": "wecom-secret"},
			publicKey:   "bot_id",
			publicValue: "wecom-bot",
		},
	}
	for _, tc := range cases {
		t.Run(tc.channelType, func(t *testing.T) {
			db := newChannelTestDB(t)
			defer db.Close()
			service := NewControlService(config.Config{
				DatabaseDriver:          "sqlite",
				ConnectorCredentialsKey: testChannelCredentialKey(),
			}, db, nil, nil)
			service.registrationPollInterval = time.Millisecond
			service.registrationClientFactory = func(string) appregistration.Client {
				return &fakeChannelRegistrationClient{credentials: tc.credentials}
			}
			if _, err := service.UpsertChannelConfig(context.Background(), "owner-a", tc.channelType, UpsertChannelConfigRequest{
				AgentID: "agent-a",
			}); err != nil {
				t.Fatalf("保存扫码前配置失败: %v", err)
			}
			started, err := service.StartChannelLogin(context.Background(), "owner-a", tc.channelType)
			if err != nil {
				t.Fatalf("启动官方扫码注册失败: %v", err)
			}
			if started.QRPayload != "https://platform.test/scan" {
				t.Fatalf("官方二维码不正确: %+v", started)
			}
			waitChannelLoginStatus(t, service, "owner-a", tc.channelType, started.LoginID, ChannelLoginStatusSucceeded)
			view, err := service.channelView(context.Background(), "owner-a", tc.channelType)
			if err != nil {
				t.Fatalf("读取扫码后频道失败: %v", err)
			}
			if !view.HasCredentials || view.PublicConfig[tc.publicKey] != tc.publicValue {
				t.Fatalf("扫码凭据未保存: %+v", view)
			}
		})
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
		{Status: "confirmed", BotToken: "token-1", IlinkBotID: "wx-account-1@im.bot", IlinkUserID: "wx-user-1@im.wechat"},
		{Status: "confirmed", BotToken: "token-2", IlinkBotID: "wx-account-2@im.bot", IlinkUserID: "wx-user-2@im.wechat"},
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
	if _, err = db.Exec(`
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, account_id, chat_type,
    external_ref, agent_id, status, source
) VALUES
    ('pairing-wx-1', 'owner-a', 'weixin-personal', 'wx-account-1@im.bot', 'dm', 'chat-1', 'agent-a', 'active', 'manual'),
    ('pairing-wx-2', 'owner-a', 'weixin-personal', 'wx-account-2@im.bot', 'dm', 'chat-2', 'agent-a', 'active', 'manual')
`); err != nil {
		t.Fatalf("准备账号 pairing 失败: %v", err)
	}
	if err = service.DeletePairing(context.Background(), "owner-a", "pairing-wx-1"); err != nil {
		t.Fatalf("先解除第一个微信账号配对失败: %v", err)
	}

	updated, err := service.DeleteChannelAccount(context.Background(), "owner-a", ChannelTypeWeixinPersonal, "wx-account-1@im.bot")
	if err != nil {
		t.Fatalf("删除单个微信账号失败: %v", err)
	}
	if updated == nil || len(updated.Accounts) != 1 || updated.Accounts[0].AccountID != "wx-account-2@im.bot" {
		t.Fatalf("删除后应只保留第二个微信账号: %+v", updated)
	}
	accounts, err := service.listChannelAccountRows(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("读取账号表失败: %v", err)
	}
	if len(accounts) != 1 || accounts[0].AccountID != "wx-account-2@im.bot" {
		t.Fatalf("账号表删除结果不正确: %+v", accounts)
	}
	var deletedPairings int
	if err = db.QueryRow(`
SELECT COUNT(*) FROM im_pairings
WHERE owner_user_id = 'owner-a' AND channel_type = 'weixin-personal'
  AND account_id = 'wx-account-1@im.bot'
`).Scan(&deletedPairings); err != nil {
		t.Fatalf("读取已删账号 pairing 失败: %v", err)
	}
	if deletedPairings != 0 {
		t.Fatalf("删除账号后仍残留 pairing: %d", deletedPairings)
	}
	var remainingPairings int
	if err = db.QueryRow(`
SELECT COUNT(*) FROM im_pairings
WHERE owner_user_id = 'owner-a' AND channel_type = 'weixin-personal'
  AND account_id = 'wx-account-2@im.bot'
`).Scan(&remainingPairings); err != nil {
		t.Fatalf("读取保留账号 pairing 失败: %v", err)
	}
	if remainingPairings != 1 {
		t.Fatalf("删除第一个账号不应影响第二个账号 pairing: %d", remainingPairings)
	}
}

func TestControlServiceDeletesLastWeixinAccountWhenNotifyStopFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/ilink/bot/msg/notifystart":
			_, _ = writer.Write([]byte(`{"ret":0}`))
		case "/ilink/bot/getupdates":
			select {
			case <-request.Context().Done():
			case <-time.After(25 * time.Millisecond):
				_, _ = writer.Write([]byte(`{"ret":0,"get_updates_buf":"cursor-1","longpolling_timeout_ms":25}`))
			}
		case "/ilink/bot/msg/notifystop":
			_, _ = writer.Write([]byte(`{"ret":-14,"errcode":-14,"errmsg":"session timeout"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	db := newChannelTestDB(t)
	defer db.Close()
	cfg := config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}
	router := NewRouter(cfg, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 Router 失败: %v", err)
	}
	defer router.Stop(context.Background())
	service := NewControlService(cfg, db, nil, router)
	service.SetHTTPClient(server.Client())
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		return &fakePersonalWeixinLoginClient{status: channeladapters.PersonalWeixinQRStatusResponse{
			Status:      "confirmed",
			BotToken:    "token-expired-on-stop",
			IlinkBotID:  "wx-account-last",
			IlinkUserID: "wx-user-last",
		}}
	}
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		UpsertChannelConfigRequest{
			AgentID: "agent-a",
			Config:  map[string]string{"base_url": server.URL},
		},
	); err != nil {
		t.Fatalf("配置个人微信通道失败: %v", err)
	}
	started, err := service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil {
		t.Fatalf("启动个人微信扫码登录失败: %v", err)
	}
	waitChannelLoginStatus(t, service, "owner-a", ChannelTypeWeixinPersonal, started.LoginID, ChannelLoginStatusSucceeded)
	deadline := time.Now().Add(2 * time.Second)
	for !router.IsReadyForOwner("owner-a", ChannelTypeWeixinPersonal) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !router.IsReadyForOwner("owner-a", ChannelTypeWeixinPersonal) {
		t.Fatal("个人微信 runtime 未完成启动通知")
	}

	updated, err := service.DeleteChannelAccount(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		"wx-account-last",
	)
	if err != nil {
		t.Fatalf("notifystop 失败不应阻断账号删除: %v", err)
	}
	if updated == nil || len(updated.Accounts) != 0 {
		t.Fatalf("删除最后账号后的视图不正确: %+v", updated)
	}
	accounts, err := service.listChannelAccountRows(context.Background(), "owner-a", ChannelTypeWeixinPersonal)
	if err != nil || len(accounts) != 0 {
		t.Fatalf("最后账号没有从数据库删除: accounts=%+v err=%v", accounts, err)
	}
	if router.GetForOwner("owner-a", ChannelTypeWeixinPersonal) != nil {
		t.Fatal("删除最后账号后 Router 仍保留旧通道")
	}
}

func TestControlServiceRejectsUnsupportedChannelLogin(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()

	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	_, err := service.StartChannelLogin(context.Background(), "owner-a", ChannelTypeTelegram)
	if !errors.Is(err, ErrChannelLoginUnsupported) {
		t.Fatalf("Telegram 不支持官方扫码注册，实际: %v", err)
	}
}
