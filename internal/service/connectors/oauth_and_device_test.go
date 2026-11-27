// oauth_and_device_test.go 覆盖连接器设备流与 OAuth 配置回调测试。
package connectors

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
	"github.com/nexus-research-lab/nexus/internal/service/auth"
	_ "modernc.org/sqlite"
)

type fakeFeishuAppRegistrationClient struct{}

func (fakeFeishuAppRegistrationClient) Start(context.Context) (*appregistration.StartResult, error) {
	return &appregistration.StartResult{
		DeviceCode:              "app-device-code",
		VerificationURIComplete: "https://accounts.feishu.test/app-registration",
		ExpiresIn:               600,
		Interval:                1,
	}, nil
}

func (fakeFeishuAppRegistrationClient) Poll(context.Context, string) (*appregistration.PollResult, error) {
	return &appregistration.PollResult{
		Status: appregistration.StatusSucceeded,
		Credentials: map[string]string{
			"client_id":     "auto-feishu-client",
			"client_secret": "auto-feishu-secret",
		},
	}, nil
}

func TestServiceDesktopGitHubDeviceFlowDisabledMessage(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.AppMode = "desktop"
	cfg.ConnectorGitHubClientSecret = ""
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, `{"error":"device_flow_disabled","error_description":"Device Flow must be explicitly enabled for this App"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL", server.URL)

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	_, err = service.StartDeviceAuth(context.Background(), auth.SystemUserID, "github", "")
	if err == nil || !strings.Contains(err.Error(), "未启用 Device Flow") {
		t.Fatalf("device_flow_disabled 应转成可读错误，实际: %v", err)
	}
}

func TestServiceFeishuDocxOfficialQRSelectsAppThenAuthorizes(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("解析飞书 Device Flow 请求失败: %v", err)
		}
		switch request.URL.Path {
		case "/device":
			clientID, clientSecret, ok := request.BasicAuth()
			if !ok || clientID != "auto-feishu-client" || clientSecret != "auto-feishu-secret" {
				t.Fatalf("飞书设备授权未使用自动创建的应用凭据")
			}
			if !strings.Contains(request.Form.Get("scope"), "docx:document") {
				t.Fatalf("飞书设备授权缺少文档权限: %v", request.Form)
			}
			_, _ = writer.Write([]byte(`{"device_code":"user-device-code","user_code":"FS-1234","verification_uri":"https://accounts.feishu.test/device","verification_uri_complete":"https://accounts.feishu.test/device?code=FS-1234","expires_in":600,"interval":1}`))
		case "/token":
			if request.Form.Get("client_id") != "auto-feishu-client" ||
				request.Form.Get("client_secret") != "auto-feishu-secret" ||
				request.Form.Get("device_code") != "user-device-code" {
				t.Fatalf("飞书 token 轮询参数不正确: %v", request.Form)
			}
			_, _ = writer.Write([]byte(`{"access_token":"feishu-device-token","refresh_token":"feishu-refresh","expires_in":7200,"scope":"docx:document"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_DEVICE_CODE_URL", server.URL+"/device")
	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL", server.URL+"/token")

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	service.registrationClientFactory = func() appregistration.Client {
		return fakeFeishuAppRegistrationClient{}
	}
	ctx := context.Background()
	const ownerUserID = "owner-feishu"
	if _, err = service.SaveOAuthClientConfig(ctx, ownerUserID, "feishu-docx", OAuthClientConfigRequest{
		ClientID:     "stale-feishu-client",
		ClientSecret: "stale-feishu-secret",
	}); err != nil {
		t.Fatalf("保存待替换的飞书应用配置失败: %v", err)
	}
	if err = service.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: "feishu-docx",
		State:       "connected",
		Credentials: `{"access_token":"stale-feishu-token","refresh_token":"stale-refresh","expires_at":4102444800}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("保存待替换的飞书连接失败: %v", err)
	}
	beforeSwitch, err := service.GetConfigurationState(
		ctx, ownerUserID, "feishu-docx",
	)
	if err != nil {
		t.Fatalf("读取切换前配置版本失败: %v", err)
	}
	if _, err = service.StartDeviceAuth(ctx, ownerUserID, "feishu-docx", ""); err == nil ||
		!strings.Contains(err.Error(), "请选择官方扫码连接或手工应用凭据兜底") {
		t.Fatalf("飞书不得静默复用已保存的应用配置: %v", err)
	}

	started, err := service.StartDeviceAuth(
		ctx,
		ownerUserID,
		"feishu-docx",
		DeviceAuthStartModeOfficialQR,
	)
	if err != nil {
		t.Fatalf("启动飞书应用扫码配置失败: %v", err)
	}
	if started.Stage != deviceAuthStageAppSelection ||
		!strings.HasPrefix(started.DeviceCode, feishuAppRegistrationDevicePrefix) {
		t.Fatalf("首阶段应为飞书应用选择或创建: %+v", started)
	}
	preservedConfig, preserveErr := service.GetOAuthClientConfig(
		ctx, ownerUserID, "feishu-docx",
	)
	if preserveErr != nil {
		t.Fatalf("读取扫码启动后的既有应用配置失败: %v", preserveErr)
	}
	if preservedConfig == nil || !preservedConfig.Configured ||
		preservedConfig.ClientID != "stale-feishu-client" {
		t.Fatalf("仅启动扫码不得提前删除既有应用配置: %+v", preservedConfig)
	}
	continued, err := service.PollDeviceAuth(ctx, ownerUserID, "feishu-docx", started.DeviceCode)
	if err != nil {
		t.Fatalf("完成飞书应用配置失败: %v", err)
	}
	if continued.Next == nil ||
		continued.Next.Stage != deviceAuthStageUserAuthorization ||
		!strings.HasPrefix(continued.Next.DeviceCode, deviceAuthAttemptPrefix) {
		t.Fatalf("应用配置后应自动进入用户扫码授权: %+v", continued)
	}
	stillOldConfig, preserveErr := service.GetOAuthClientConfig(
		ctx, ownerUserID, "feishu-docx",
	)
	stillOldConnection, connectionErr := service.LoadActiveConnection(
		ctx, ownerUserID, "feishu-docx",
	)
	afterAppStage, stateErr := service.GetConfigurationState(
		ctx, ownerUserID, "feishu-docx",
	)
	if preserveErr != nil || connectionErr != nil || stateErr != nil ||
		stillOldConfig == nil || stillOldConfig.ClientID != "stale-feishu-client" ||
		stillOldConnection == nil || stillOldConnection.AccessToken != "stale-feishu-token" ||
		afterAppStage.ConfigurationVersion != beforeSwitch.ConfigurationVersion {
		t.Fatalf(
			"用户授权成功前不得切换 canonical client/connection: config=%+v connection=%+v state=%+v errors=%v/%v/%v",
			stillOldConfig, stillOldConnection, afterAppStage,
			preserveErr, connectionErr, stateErr,
		)
	}
	connected, err := service.PollDeviceAuth(ctx, ownerUserID, "feishu-docx", continued.Next.DeviceCode)
	if err != nil {
		t.Fatalf("完成飞书云文档授权失败: %v", err)
	}
	if connected.Status != deviceAuthStatusConnected {
		t.Fatalf("飞书云文档未连接: %+v", connected)
	}
	snapshot, err := service.LoadActiveConnection(ctx, ownerUserID, "feishu-docx")
	if err != nil || snapshot == nil || snapshot.AccessToken != "feishu-device-token" {
		t.Fatalf("飞书云文档 token 未保存: snapshot=%+v err=%v", snapshot, err)
	}
	switchedConfig, configErr := service.GetOAuthClientConfig(
		ctx, ownerUserID, "feishu-docx",
	)
	afterSwitch, stateErr := service.GetConfigurationState(
		ctx, ownerUserID, "feishu-docx",
	)
	if configErr != nil || stateErr != nil || switchedConfig == nil ||
		switchedConfig.ClientID != "auto-feishu-client" ||
		afterSwitch.ConfigurationVersion != beforeSwitch.ConfigurationVersion+1 {
		t.Fatalf(
			"成功时应在一个版本中切换 client/connection: config=%+v state=%+v errors=%v/%v",
			switchedConfig, afterSwitch, configErr, stateErr,
		)
	}
	if _, err = db.Exec(`
INSERT INTO connector_oauth_states (
    state, owner_user_id, connector_id, code_verifier, redirect_uri, redirect_kind, expires_at
) VALUES (
    'stale-oauth-state', ?, 'feishu-docx', 'verifier', 'http://localhost/callback', 'web',
    datetime('now', '+10 minutes')
)`, ownerUserID); err != nil {
		t.Fatalf("准备待清理 OAuth state 失败: %v", err)
	}
	disconnected, err := service.Disconnect(ctx, ownerUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("断开飞书云文档失败: %v", err)
	}
	if disconnected.ConnectionState != "disconnected" || disconnected.OAuthClientConfigured {
		t.Fatalf("断开后应同时清除用户 token 与应用配置: %+v", disconnected)
	}
	snapshot, err = service.LoadActiveConnection(ctx, ownerUserID, "feishu-docx")
	if err != nil || snapshot != nil {
		t.Fatalf("断开后不应保留飞书连接 token: snapshot=%+v err=%v", snapshot, err)
	}
	config, err := service.GetOAuthClientConfig(ctx, ownerUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取断开后的飞书应用配置失败: %v", err)
	}
	if config == nil || config.Configured || config.ClientID != "" {
		t.Fatalf("断开后不应保留固定 App ID / Secret: %+v", config)
	}
	for tableIndex, query := range []string{
		"SELECT COUNT(*) FROM connector_oauth_states WHERE owner_user_id = ? AND connector_id = 'feishu-docx'",
		"SELECT COUNT(*) FROM connector_connections WHERE owner_user_id = ? AND connector_id = 'feishu-docx'",
		"SELECT COUNT(*) FROM connector_oauth_clients WHERE owner_user_id = ? AND connector_id = 'feishu-docx'",
	} {
		var count int
		if err = db.QueryRow(query, ownerUserID).Scan(&count); err != nil {
			t.Fatalf("读取连接器删除残留[%d]失败: %v", tableIndex, err)
		}
		if count != 0 {
			t.Fatalf("连接器删除后仍残留表数据[%d]: %d", tableIndex, count)
		}
	}
	if _, err = service.StartDeviceAuth(
		ctx,
		ownerUserID,
		"feishu-docx",
		DeviceAuthStartModeManualCredentials,
	); err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("手工兜底不得复用断开前的应用配置: %v", err)
	}
	replacement, err := service.StartDeviceAuth(
		ctx,
		ownerUserID,
		"feishu-docx",
		DeviceAuthStartModeOfficialQR,
	)
	if err != nil {
		t.Fatalf("断开后重新启动官方扫码配置失败: %v", err)
	}
	if replacement.Stage != deviceAuthStageAppSelection ||
		!strings.HasPrefix(replacement.DeviceCode, feishuAppRegistrationDevicePrefix) {
		t.Fatalf("断开后应重新选择或创建飞书应用: %+v", replacement)
	}
}

func TestServiceRejectsRedirectURIOutsideAllowedOrigins(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	_, err = service.GetAuthURL(context.Background(), auth.SystemUserID, "github", "https://evil.example/callback", nil)
	if err == nil || !strings.Contains(err.Error(), "允许列表") {
		t.Fatalf("应拒绝非白名单 redirect URI，实际: %v", err)
	}
}
