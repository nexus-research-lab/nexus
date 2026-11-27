// oauth_and_device_test.go 覆盖连接器设备流与 OAuth 配置回调测试。
package connectors

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestServiceDesktopGitHubDeviceFlowUsesPublicClientID(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.AppMode = "desktop"
	cfg.ConnectorGitHubClientSecret = ""
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	tokenPollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("解析 GitHub device 请求失败: %v", err)
		}
		if request.Form.Get("client_id") != cfg.ConnectorGitHubClientID {
			t.Fatalf("device flow 未使用公开 client_id: %v", request.Form)
		}
		if request.Form.Get("client_secret") != "" {
			t.Fatalf("device flow 不应发送 client_secret: %v", request.Form)
		}
		switch request.URL.Path {
		case "/device":
			_, _ = writer.Write([]byte(`{"device_code":"device-code","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":1}`))
		case "/token":
			tokenPollCount++
			if request.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
				t.Fatalf("grant_type 不正确: %v", request.Form)
			}
			if request.Form.Get("device_code") != "device-code" {
				t.Fatalf("device_code 不正确: %v", request.Form)
			}
			if tokenPollCount == 1 {
				_, _ = writer.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"github-device-token","scope":"repo","token_type":"bearer"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL", server.URL+"/device")
	t.Setenv("NEXUS_CONNECTOR_GITHUB_TOKEN_URL", server.URL+"/token")

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	ctx := context.Background()

	items, err := service.ListConnectors(ctx, auth.SystemUserID, "github", "", "")
	if err != nil {
		t.Fatalf("列出连接器失败: %v", err)
	}
	if len(items) != 1 || !items[0].IsConfigured {
		t.Fatalf("桌面 GitHub 只配置 client_id 时应可用: %+v", items)
	}

	start, err := service.StartDeviceAuth(ctx, auth.SystemUserID, "github", "")
	if err != nil {
		t.Fatalf("启动 GitHub device flow 失败: %v", err)
	}
	if start.UserCode != "ABCD-1234" ||
		!strings.HasPrefix(start.DeviceCode, deviceAuthAttemptPrefix) {
		t.Fatalf("device flow 启动结果不正确: %+v", start)
	}
	entry, _ := getConnector("github")
	attempt, err := service.openDeviceAuthAttempt(
		ctx, auth.SystemUserID, entry, start.DeviceCode,
	)
	if err != nil || attempt.DeviceCode != "device-code" ||
		attempt.ClientID != cfg.ConnectorGitHubClientID {
		t.Fatalf("device flow 未精确绑定启动凭据: attempt=%+v err=%v", attempt, err)
	}
	if _, err = service.PollDeviceAuth(
		ctx, "other-owner", "github", start.DeviceCode,
	); err == nil {
		t.Fatal("Device Flow attempt 不得跨 owner 使用")
	}
	if _, err = service.PollDeviceAuth(
		ctx, auth.SystemUserID, "feishu-docx", start.DeviceCode,
	); err == nil {
		t.Fatal("Device Flow attempt 不得跨 Connector 使用")
	}
	if tokenPollCount != 0 {
		t.Fatalf("身份错配的 attempt 不得调用 provider: polls=%d", tokenPollCount)
	}

	pending, err := service.PollDeviceAuth(ctx, auth.SystemUserID, "github", start.DeviceCode)
	if err != nil {
		t.Fatalf("轮询 GitHub device flow 失败: %v", err)
	}
	if pending.Status != deviceAuthStatusPending {
		t.Fatalf("首次轮询应为 pending: %+v", pending)
	}

	connected, err := service.PollDeviceAuth(ctx, auth.SystemUserID, "github", start.DeviceCode)
	if err != nil {
		t.Fatalf("完成 GitHub device flow 失败: %v", err)
	}
	if connected.Status != deviceAuthStatusConnected || connected.Connector == nil || connected.Connector.ConnectionState != "connected" {
		t.Fatalf("device flow 未完成连接: %+v", connected)
	}
	snapshot, err := service.LoadActiveConnection(ctx, auth.SystemUserID, "github")
	if err != nil {
		t.Fatalf("读取 GitHub 连接失败: %v", err)
	}
	if snapshot == nil || snapshot.AccessToken != "github-device-token" {
		t.Fatalf("GitHub token 未保存: %+v", snapshot)
	}
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

func TestServiceFeishuDocxManualCredentialsAreFallbackOnly(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/device":
			clientID, clientSecret, ok := request.BasicAuth()
			if !ok {
				t.Fatal("飞书手工兜底未使用 HTTP Basic 应用凭据")
			}
			if clientID != "manual-feishu-client" || clientSecret != "manual-feishu-secret" {
				http.Error(writer, `{"error":"invalid_client"}`, http.StatusUnauthorized)
				return
			}
			_, _ = writer.Write([]byte(`{"device_code":"manual-device-code","user_code":"MANUAL-1234","verification_uri":"https://accounts.feishu.test/device","expires_in":600,"interval":1}`))
		case "/token":
			_, _ = writer.Write([]byte(`{"error":"access_denied"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_DEVICE_CODE_URL", server.URL+"/device")
	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL", server.URL+"/token")

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	ctx := context.Background()
	const ownerUserID = "owner-feishu-manual"
	if _, err = service.SaveOAuthClientConfig(ctx, ownerUserID, "feishu-docx", OAuthClientConfigRequest{
		ClientID:     "manual-feishu-client",
		ClientSecret: "manual-feishu-secret",
	}); err != nil {
		t.Fatalf("保存手工兜底应用配置失败: %v", err)
	}
	started, err := service.StartDeviceAuth(
		ctx,
		ownerUserID,
		"feishu-docx",
		DeviceAuthStartModeManualCredentials,
	)
	if err != nil {
		t.Fatalf("使用手工兜底应用配置启动授权失败: %v", err)
	}
	if started.Stage != deviceAuthStageUserAuthorization ||
		!strings.HasPrefix(started.DeviceCode, deviceAuthAttemptPrefix) {
		t.Fatalf("手工兜底应直接进入用户授权: %+v", started)
	}
	denied, err := service.PollDeviceAuth(
		ctx, ownerUserID, "feishu-docx", started.DeviceCode,
	)
	if err != nil || denied.Status != deviceAuthStatusDenied {
		t.Fatalf("飞书拒绝授权状态不正确: result=%+v err=%v", denied, err)
	}
	preservedAfterDenial, err := service.GetOAuthClientConfig(
		ctx, ownerUserID, "feishu-docx",
	)
	if err != nil || preservedAfterDenial == nil ||
		!preservedAfterDenial.Configured ||
		preservedAfterDenial.ClientID != "manual-feishu-client" {
		t.Fatalf(
			"用户拒绝临时授权不得删除应用配置: config=%+v err=%v",
			preservedAfterDenial, err,
		)
	}

	if _, err = service.SaveOAuthClientConfig(ctx, ownerUserID, "feishu-docx", OAuthClientConfigRequest{
		ClientID:     "invalid-feishu-client",
		ClientSecret: "invalid-feishu-secret",
	}); err != nil {
		t.Fatalf("保存无效手工兜底应用配置失败: %v", err)
	}
	if _, err = service.StartDeviceAuth(
		ctx,
		ownerUserID,
		"feishu-docx",
		DeviceAuthStartModeManualCredentials,
	); err == nil {
		t.Fatal("无效手工兜底应用配置不应启动成功")
	}
	config, configErr := service.GetOAuthClientConfig(ctx, ownerUserID, "feishu-docx")
	if configErr != nil {
		t.Fatalf("读取失败后的手工应用配置失败: %v", configErr)
	}
	if config == nil || !config.Configured || config.ClientID != "invalid-feishu-client" {
		t.Fatalf("手工兜底启动失败不得删除已保存的 App ID / Secret: %+v", config)
	}
}

func TestServiceFeishuDocxUsesUserOAuthClientConfig(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("读取 token 请求失败: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"client_id":"user-feishu-client"`) || !strings.Contains(text, `"client_secret":"user-feishu-secret"`) {
			t.Fatalf("飞书 token 交换未使用用户自有 OAuth Client: %s", body)
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"access_token":"feishu-token","refresh_token":"refresh","expires_in":7200}}`))
	}))
	defer server.Close()
	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL", server.URL)

	service := NewService(cfg, db)
	service.httpClient = server.Client()
	ctx := context.Background()
	const ownerUserID = "user-feishu-docx"

	items, err := service.ListConnectors(ctx, ownerUserID, "feishu", "", "")
	if err != nil {
		t.Fatalf("列出飞书连接器失败: %v", err)
	}
	if len(items) != 1 || !items[0].IsConfigured || items[0].OAuthClientConfigRequired || !items[0].SupportsDeviceAuth {
		t.Fatalf("飞书云文档应可直接进入自动扫码授权: %+v", items)
	}
	if items[0].ConfigError != nil {
		t.Fatalf("自动扫码创建应用前不应暴露配置错误: %+v", items[0].ConfigError)
	}
	if _, err = service.GetAuthURL(ctx, ownerUserID, "feishu-docx", "", nil); err == nil {
		t.Fatalf("未保存用户 OAuth Client 前不应生成授权地址")
	}

	info, err := service.SaveOAuthClientConfig(ctx, ownerUserID, "feishu-docx", OAuthClientConfigRequest{
		ClientID:     "user-feishu-client",
		ClientSecret: "user-feishu-secret",
	})
	if err != nil {
		t.Fatalf("保存用户 OAuth Client 失败: %v", err)
	}
	if !info.IsConfigured {
		t.Fatalf("保存手工兜底凭据后应视为已配置: %+v", info)
	}
	detail, err := service.GetConnectorDetail(ctx, ownerUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取飞书详情失败: %v", err)
	}
	if detail.OAuthClientID == nil || *detail.OAuthClientID != "user-feishu-client" {
		t.Fatalf("详情应返回已保存的 Client ID 摘要: %+v", detail.OAuthClientID)
	}
	if len(detail.FeatureDetails) != len(detail.Features) {
		t.Fatalf("详情应返回每个能力的具体说明: features=%v details=%v", detail.Features, detail.FeatureDetails)
	}
	if detail.FeatureDetails[0].Name != "阅读文档" || !strings.Contains(detail.FeatureDetails[0].Description, "Markdown") {
		t.Fatalf("阅读文档能力说明不完整: %+v", detail.FeatureDetails[0])
	}

	authURL, err := service.GetAuthURL(ctx, ownerUserID, "feishu-docx", "", nil)
	if err != nil {
		t.Fatalf("生成飞书授权地址失败: %v", err)
	}
	parsedURL, err := url.Parse(authURL.AuthURL)
	if err != nil {
		t.Fatalf("解析飞书授权地址失败: %v", err)
	}
	if parsedURL.Query().Get("client_id") != "user-feishu-client" {
		t.Fatalf("飞书授权地址应使用用户 Client ID: %s", authURL.AuthURL)
	}

	callback, err := service.CompleteOAuthCallback(ctx, ownerUserID, OAuthCallbackRequest{
		Code:  "callback-code",
		State: authURL.State,
	})
	if err != nil {
		t.Fatalf("飞书 OAuth callback 失败: %v", err)
	}
	if callback == nil || callback.ConnectionState != "connected" {
		t.Fatalf("飞书 OAuth callback 后应连接成功: %+v", callback)
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
