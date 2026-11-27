// control_login_test.go 覆盖渠道登录流程与安全边界测试。
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

type blockedPersonalWeixinLoginClient struct {
	started chan struct{}
	release chan struct{}
	status  channeladapters.PersonalWeixinQRStatusResponse
}

type rejectingChannelLoginAuthorizationCommitGuard struct {
	requests chan ChannelLoginAuthorizationCommit
}

func (g *rejectingChannelLoginAuthorizationCommitGuard) AcquireChannelLoginAuthorizationCommit(
	_ context.Context,
	request ChannelLoginAuthorizationCommit,
) (func(), error) {
	if g != nil && g.requests != nil {
		g.requests <- request
	}
	return nil, errors.New("bound human session revoked")
}

func (c *blockedPersonalWeixinLoginClient) StartQRCode(
	context.Context,
	[]string,
) (channeladapters.PersonalWeixinQRCodeResponse, error) {
	return channeladapters.PersonalWeixinQRCodeResponse{
		QRCode:             "blocked-qr-token",
		QRCodeImageContent: "weixin://blocked-qr",
	}, nil
}

func (c *blockedPersonalWeixinLoginClient) PollQRCodeStatus(
	ctx context.Context,
	_ string,
	_ string,
) (channeladapters.PersonalWeixinQRStatusResponse, error) {
	select {
	case c.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return channeladapters.PersonalWeixinQRStatusResponse{}, ctx.Err()
	case <-c.release:
		return c.status, nil
	}
}

func TestChannelLoginCompletionRejectsChangedControlVersion(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	client := &blockedPersonalWeixinLoginClient{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		status: channeladapters.PersonalWeixinQRStatusResponse{
			Status:      "confirmed",
			BotToken:    "stale-login-token",
			IlinkBotID:  "stale-account",
			IlinkUserID: "stale-user",
			BaseURL:     "https://stale-login.example",
		},
	}
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		return client
	}
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		UpsertChannelConfigRequest{
			AgentID: "agent-a",
			Config:  map[string]string{"base_url": "https://before.example"},
		},
	); err != nil {
		t.Fatal(err)
	}
	startVersion, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || startVersion != 2 {
		t.Fatalf("start version=%d err=%v", startVersion, err)
	}
	started, err := service.StartChannelLoginAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		startVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("login poll did not start")
	}
	if _, err = service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		UpsertChannelConfigRequest{
			AgentID: "agent-a",
			Config:  map[string]string{"base_url": "https://newer.example"},
		},
	); err != nil {
		t.Fatal(err)
	}
	failed := waitChannelLoginStatus(
		t,
		service,
		"owner-a",
		ChannelTypeWeixinPersonal,
		started.LoginID,
		ChannelLoginStatusCancelled,
	)
	if failed.QRPayload != "" || failed.QRPayloadType != "" {
		t.Fatalf("配置重绑后取消的登录仍保留二维码: %+v", failed)
	}
	row, err := service.getChannelConfigRow(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatal(err)
	}
	values, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		t.Fatal(err)
	}
	if values["base_url"] != "https://newer.example" {
		t.Fatalf("stale login overwrote newer config: %+v", values)
	}
	accounts, err := service.listChannelAccountRows(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 0 {
		t.Fatalf("stale login persisted account: %+v", accounts)
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != 3 {
		t.Fatalf("newer version changed: version=%d err=%v", version, err)
	}
}

func TestChannelLoginRejectsUnexpectedAccountBeforeCredentialWrite(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		return &fakePersonalWeixinLoginClient{status: channeladapters.PersonalWeixinQRStatusResponse{
			Status:      "confirmed",
			BotToken:    "wrong-account-token",
			IlinkBotID:  "unexpected-account",
			IlinkUserID: "unexpected-user",
		}}
	}
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		UpsertChannelConfigRequest{AgentID: "agent-a"},
	); err != nil {
		t.Fatal(err)
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartChannelLoginForAccountAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		"expected-account",
		version,
	)
	if err != nil {
		t.Fatal(err)
	}
	failed := waitChannelLoginStatus(
		t,
		service,
		"owner-a",
		ChannelTypeWeixinPersonal,
		started.LoginID,
		ChannelLoginStatusError,
	)
	if !strings.Contains(failed.Error, "账号与授权目标不匹配") {
		t.Fatalf("account mismatch failure = %+v", failed)
	}
	accounts, err := service.listChannelAccountRows(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 0 {
		t.Fatalf("unexpected account credential was persisted: %+v", accounts)
	}
}

func TestChannelLoginAuthorizationGenerationCannotAdoptOrCancelAnotherFlow(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	client := &blockedPersonalWeixinLoginClient{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		status: channeladapters.PersonalWeixinQRStatusResponse{
			Status: "confirmed", BotToken: "token", IlinkBotID: "account",
		},
	}
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		return client
	}
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		UpsertChannelConfigRequest{AgentID: "agent-a"},
	); err != nil {
		t.Fatal(err)
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.StartChannelLoginForAuthorizationAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		"",
		"flow-generation-a",
		version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartChannelLoginForAuthorizationAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		"",
		"flow-generation-b",
		version,
	); !errors.Is(err, ErrChannelLoginState) {
		t.Fatalf("cross-generation adoption must fail: %v", err)
	}
	view, err := service.GetChannelLogin(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		first.LoginID,
	)
	if err != nil || view.Status != ChannelLoginStatusRunning {
		t.Fatalf("cross-generation start cancelled original flow: view=%+v err=%v", view, err)
	}
	_, _ = service.CancelChannelLoginAndWait(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		first.LoginID,
	)
}

func TestChannelLoginAuthorizationGuardRejectsCredentialCommit(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	client := &blockedPersonalWeixinLoginClient{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		status: channeladapters.PersonalWeixinQRStatusResponse{
			Status:      "confirmed",
			BotToken:    "must-not-persist-token",
			IlinkBotID:  "must-not-persist-account",
			IlinkUserID: "must-not-persist-user",
		},
	}
	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, nil)
	service.weixinLoginClientFactory = func(string, map[string]string) personalWeixinLoginClient {
		return client
	}
	guard := &rejectingChannelLoginAuthorizationCommitGuard{
		requests: make(chan ChannelLoginAuthorizationCommit, 1),
	}
	service.SetChannelLoginAuthorizationCommitGuard(guard)
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		UpsertChannelConfigRequest{AgentID: "agent-a"},
	); err != nil {
		t.Fatal(err)
	}
	startVersion, err := service.GetChannelControlVersion(
		context.Background(),
		"owner-a",
	)
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartChannelLoginForAuthorizationAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		"",
		"flow-generation-revoked",
		startVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("login poll did not start")
	}
	close(client.release)
	select {
	case request := <-guard.requests:
		if request.OwnerUserID != "owner-a" ||
			request.ChannelType != ChannelTypeWeixinPersonal ||
			request.LoginID != started.LoginID ||
			request.AuthorizationBinding != "flow-generation-revoked" ||
			request.StartControlVersion != startVersion {
			t.Fatalf("authorization commit request = %+v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("authorization commit guard was not called")
	}
	failed := waitChannelLoginStatus(
		t,
		service,
		"owner-a",
		ChannelTypeWeixinPersonal,
		started.LoginID,
		ChannelLoginStatusError,
	)
	if !strings.Contains(
		failed.Error,
		ErrChannelLoginAuthorizationCommit.Error(),
	) {
		t.Fatalf("authorization rejection status = %+v", failed)
	}
	accounts, err := service.listChannelAccountRows(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 0 {
		t.Fatalf("revoked authorization persisted account: %+v", accounts)
	}
	version, err := service.GetChannelControlVersion(
		context.Background(),
		"owner-a",
	)
	if err != nil || version != startVersion {
		t.Fatalf(
			"revoked authorization changed version: version=%d start=%d err=%v",
			version,
			startVersion,
			err,
		)
	}
}

func TestChannelLoginRuntimeFailureRestoresPriorConfigAtNewVersion(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())

	good := &recordingDeliveryChannel{channelType: ChannelTypeFeishu}
	failing := &recordingDeliveryChannel{
		channelType: ChannelTypeFeishu,
		startErr:    errTestChannelLoginReload,
	}
	current := DeliveryChannel(good)
	previous := routerChannelConfigurers[ChannelTypeFeishu]
	routerChannelConfigurers[ChannelTypeFeishu] = func(
		service *ControlService,
		ctx context.Context,
		cfg routerChannelConfiguration,
	) error {
		return service.registerConfiguredChannel(ctx, cfg, current)
	}
	t.Cleanup(func() { routerChannelConfigurers[ChannelTypeFeishu] = previous })

	service := NewControlService(config.Config{
		DatabaseDriver:          "sqlite",
		ConnectorCredentialsKey: testChannelCredentialKey(),
	}, db, nil, router)
	service.registrationPollInterval = time.Millisecond
	service.registrationClientFactory = func(string) appregistration.Client {
		return &fakeChannelRegistrationClient{credentials: map[string]string{
			"client_id": "new-app-id", "client_secret": "new-app-secret",
		}}
	}
	if _, err := service.UpsertChannelConfig(
		context.Background(),
		"owner-a",
		ChannelTypeFeishu,
		UpsertChannelConfigRequest{
			AgentID:     "agent-a",
			Config:      map[string]string{"app_id": "old-app-id"},
			Credentials: map[string]string{"app_secret": "old-app-secret"},
		},
	); err != nil {
		t.Fatal(err)
	}
	startVersion, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || startVersion != 2 {
		t.Fatalf("start version=%d err=%v", startVersion, err)
	}
	current = failing
	started, err := service.StartChannelLoginAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeFeishu,
		startVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	failed := waitChannelLoginStatus(
		t,
		service,
		"owner-a",
		ChannelTypeFeishu,
		started.LoginID,
		ChannelLoginStatusError,
	)
	if !strings.Contains(failed.Error, "上一份可运行配置已保留") {
		t.Fatalf("reload failure = %+v", failed)
	}
	if got := router.GetForOwner("owner-a", ChannelTypeFeishu); got != good {
		t.Fatalf("failed candidate replaced known-good runtime: got=%T", got)
	}
	row, err := service.getChannelConfigRow(context.Background(), "owner-a", ChannelTypeFeishu)
	if err != nil {
		t.Fatal(err)
	}
	public, err := decodeStringMap(row.ConfigJSON)
	if err != nil {
		t.Fatal(err)
	}
	secrets, err := service.decryptCredentials(row.CredentialsEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if public["app_id"] != "old-app-id" ||
		secrets["app_secret"] != "old-app-secret" {
		t.Fatalf("failed login polluted config: public=%v secrets=%v", public, secrets)
	}
	version, err := service.GetChannelControlVersion(context.Background(), "owner-a")
	if err != nil || version != startVersion+2 {
		t.Fatalf("failed login must publish restored state at a new monotonic version: version=%d err=%v", version, err)
	}
}

var errTestChannelLoginReload = &testChannelLoginError{"authorization candidate failed"}

type testChannelLoginError struct {
	message string
}

func (e *testChannelLoginError) Error() string {
	return e.message
}
