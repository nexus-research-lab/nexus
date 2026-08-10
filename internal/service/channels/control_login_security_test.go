package channels

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

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
	close(client.release)
	failed := waitChannelLoginStatus(
		t,
		service,
		"owner-a",
		ChannelTypeWeixinPersonal,
		started.LoginID,
		ChannelLoginStatusError,
	)
	if !strings.Contains(failed.Error, "配置版本已变化") {
		t.Fatalf("stale login failure = %+v", failed)
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

func TestCancelChannelLoginAndWaitDrainsExactPoller(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	client := &blockedPersonalWeixinLoginClient{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		status: channeladapters.PersonalWeixinQRStatusResponse{
			Status: "wait",
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
	version, err := service.GetChannelControlVersion(
		context.Background(),
		"owner-a",
	)
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartChannelLoginAtVersion(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		version,
	)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("login poll did not start")
	}
	session, err := service.getChannelLoginSession(
		"owner-a",
		ChannelTypeWeixinPersonal,
		started.LoginID,
	)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := service.CancelChannelLoginAndWait(
		context.Background(),
		"owner-a",
		ChannelTypeWeixinPersonal,
		started.LoginID,
	)
	if err != nil || cancelled.Status != ChannelLoginStatusCancelled {
		t.Fatalf("cancel-and-wait = %+v err=%v", cancelled, err)
	}
	select {
	case <-session.done:
	default:
		t.Fatal("CancelChannelLoginAndWait returned before poller exit")
	}
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
