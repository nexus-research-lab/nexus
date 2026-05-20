package connectors

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/service/auth"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestServiceListsConnectorsAndBuildsAuthURL(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()

	items, err := service.ListConnectors(ctx, auth.SystemUserID, "github", "", "")
	if err != nil {
		t.Fatalf("列出连接器失败: %v", err)
	}
	if len(items) != 1 || items[0].ConnectorID != "github" {
		t.Fatalf("连接器过滤结果不正确: %+v", items)
	}
	if !items[0].IsConfigured {
		t.Fatalf("GitHub 连接器应视为已配置: %+v", items[0])
	}

	authURL, err := service.GetAuthURL(ctx, auth.SystemUserID, "github", "", nil)
	if err != nil {
		t.Fatalf("生成授权地址失败: %v", err)
	}
	parsedURL, err := url.Parse(authURL.AuthURL)
	if err != nil {
		t.Fatalf("解析授权地址失败: %v", err)
	}
	if parsedURL.Query().Get("client_id") != cfg.ConnectorGitHubClientID {
		t.Fatalf("client_id 未写入授权地址: %s", authURL.AuthURL)
	}
	if strings.TrimSpace(authURL.State) == "" {
		t.Fatalf("state 不能为空: %+v", authURL)
	}

	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"token"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}
	count, err := service.GetConnectedCount(ctx)
	if err != nil {
		t.Fatalf("读取已连接数量失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("已连接数量不正确: got=%d want=1", count)
	}
}

func TestServiceOAuthUsesDeploymentCredentialsOnly(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.ConnectorGitHubClientID = ""
	cfg.ConnectorGitHubClientSecret = ""
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	const ownerUserID = "user-oauth-client"

	items, err := service.ListConnectors(ctx, ownerUserID, "github", "", "")
	if err != nil {
		t.Fatalf("列出连接器失败: %v", err)
	}
	if len(items) != 1 || items[0].IsConfigured {
		t.Fatalf("未配置环境变量时应为待配置: %+v", items)
	}

	cfg.ConnectorGitHubClientID = "env-client-id"
	cfg.ConnectorGitHubClientSecret = "env-client-secret"
	service = NewService(cfg, db)

	items, err = service.ListConnectors(ctx, ownerUserID, "github", "", "")
	if err != nil {
		t.Fatalf("列出连接器失败: %v", err)
	}
	if len(items) != 1 || !items[0].IsConfigured {
		t.Fatalf("配置环境变量后应可连接: %+v", items)
	}

	authURL, err := service.GetAuthURL(ctx, ownerUserID, "github", "", nil)
	if err != nil {
		t.Fatalf("生成授权地址失败: %v", err)
	}
	parsedURL, err := url.Parse(authURL.AuthURL)
	if err != nil {
		t.Fatalf("解析授权地址失败: %v", err)
	}
	if parsedURL.Query().Get("client_id") != "env-client-id" {
		t.Fatalf("应使用环境变量中的 client_id，实际: %s", authURL.AuthURL)
	}
}

func TestServiceShopifyRequiresShop(t *testing.T) {
	t.Skip("Shopify 目前在 catalog 中为 coming_soon，已暂停对外发布；如需恢复请先把 status 改回 available")
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()

	_, err = service.GetAuthURL(ctx, auth.SystemUserID, "shopify", "", nil)
	if err == nil || !strings.Contains(err.Error(), "shop 参数缺失") {
		t.Fatalf("expected missing shop error, got %v", err)
	}

	authURL, err := service.GetAuthURL(ctx, auth.SystemUserID, "shopify", "", map[string]string{"shop": "demo"})
	if err != nil {
		t.Fatalf("生成 Shopify 授权地址失败: %v", err)
	}
	if !strings.HasPrefix(authURL.AuthURL, "https://demo.myshopify.com/admin/oauth/authorize") {
		t.Fatalf("Shopify 授权地址未替换 shop: %s", authURL.AuthURL)
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

func TestServiceRecordsDesktopRedirectKind(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.ConnectorOAuthAllowedOrigins = []string{"http://localhost:3000", "nexus://connectors"}
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	authURL, err := service.GetAuthURL(ctx, auth.SystemUserID, "github", "nexus://connectors/oauth/callback", nil)
	if err != nil {
		t.Fatalf("生成桌面授权地址失败: %v", err)
	}

	var redirectKind string
	//goland:noinspection SqlResolve
	if err = db.QueryRowContext(ctx, "SELECT redirect_kind FROM connector_oauth_states WHERE state = ?", authURL.State).Scan(&redirectKind); err != nil {
		t.Fatalf("查询 OAuth redirect kind 失败: %v", err)
	}
	if redirectKind != oauthRedirectKindDesktop {
		t.Fatalf("redirect kind 不正确: got=%q want=%q", redirectKind, oauthRedirectKindDesktop)
	}
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

	start, err := service.StartDeviceAuth(ctx, auth.SystemUserID, "github")
	if err != nil {
		t.Fatalf("启动 GitHub device flow 失败: %v", err)
	}
	if start.UserCode != "ABCD-1234" || start.DeviceCode != "device-code" {
		t.Fatalf("device flow 启动结果不正确: %+v", start)
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
	_, err = service.StartDeviceAuth(context.Background(), auth.SystemUserID, "github")
	if err == nil || !strings.Contains(err.Error(), "未启用 Device Flow") {
		t.Fatalf("device_flow_disabled 应转成可读错误，实际: %v", err)
	}
}

func TestServiceMultipleAuthURLsDoNotOverwrite(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()

	first, err := service.GetAuthURL(ctx, auth.SystemUserID, "github", "", nil)
	if err != nil {
		t.Fatalf("生成第一次授权地址失败: %v", err)
	}
	second, err := service.GetAuthURL(ctx, auth.SystemUserID, "github", "", nil)
	if err != nil {
		t.Fatalf("生成第二次授权地址失败: %v", err)
	}
	if first.State == second.State {
		t.Fatalf("两次 state 不应相同: %q", first.State)
	}

	var count int
	//goland:noinspection SqlResolve
	if err = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM connector_oauth_states WHERE state IN (?, ?)", first.State, second.State).Scan(&count); err != nil {
		t.Fatalf("查询 OAuth state 失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("OAuth state 不应被覆盖: got=%d want=2", count)
	}
}

func TestServiceEncryptsConnectionCredentials(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := NewService(cfg, db)
	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"secret-token"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}

	var credentialText string
	var encrypted sql.NullString
	//goland:noinspection SqlResolve
	if err = db.QueryRowContext(ctx, "SELECT credentials, credentials_encrypted FROM connector_connections WHERE connector_id = ?", "github").Scan(&credentialText, &encrypted); err != nil {
		t.Fatalf("读取连接凭证失败: %v", err)
	}
	if credentialText != "__encrypted__" {
		t.Fatalf("明文字段不应保存 token payload: %q", credentialText)
	}
	if !encrypted.Valid || strings.Contains(encrypted.String, "secret-token") {
		t.Fatalf("加密字段未正确写入: %q", encrypted.String)
	}
	key, err := credentials.DecodeKey(cfg.ConnectorCredentialsKey)
	if err != nil {
		t.Fatalf("解析测试密钥失败: %v", err)
	}
	plain, err := credentials.DecryptPayload(key, encrypted.String)
	if err != nil {
		t.Fatalf("解密连接凭证失败: %v", err)
	}
	if string(plain) != `{"access_token":"secret-token"}` {
		t.Fatalf("解密后的凭证不正确: %s", plain)
	}
}

func TestServiceLoadActiveConnectionDecryptsAccessToken(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	cfg.ConnectorCredentialsKey = testConnectorCredentialKey()
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"access_token":"token","scope":"repo"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}

	item, err := service.LoadActiveConnection(ctx, auth.SystemUserID, "github")
	if err != nil {
		t.Fatalf("读取连接快照失败: %v", err)
	}
	if item == nil || item.AccessToken != "token" || item.APIBaseURL != "https://api.github.com" {
		t.Fatalf("连接快照不正确: %+v", item)
	}
	if item.Extra["scope"] != "repo" {
		t.Fatalf("extra 字段未保留: %+v", item.Extra)
	}

	items, err := service.ListActiveConnections(ctx, auth.SystemUserID)
	if err != nil {
		t.Fatalf("列出连接快照失败: %v", err)
	}
	if len(items) != 1 || items[0].ConnectorID != "github" || items[0].AccessToken != "token" {
		t.Fatalf("连接快照列表不正确: %+v", items)
	}
}

func TestServiceLoadActiveConnectionRequiresAccessToken(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	if err = service.upsertConnection(ctx, connectionRecord{
		ConnectorID: "github",
		State:       "connected",
		Credentials: `{"scope":"repo"}`,
		AuthType:    "oauth2",
	}); err != nil {
		t.Fatalf("写入连接状态失败: %v", err)
	}

	_, err = service.LoadActiveConnection(ctx, auth.SystemUserID, "github")
	if err == nil || !strings.Contains(err.Error(), "access token") {
		t.Fatalf("缺少 access token 应报错，实际: %v", err)
	}
}

func TestServiceOAuthCallbackUsesStoredVerifier(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("解析 token 请求失败: %v", err)
		}
		if request.Form.Get("code_verifier") != "stored-verifier" {
			t.Fatalf("未使用存储的 PKCE verifier: %v", request.Form)
		}
		_, _ = writer.Write([]byte(`{"access_token":"twitter-token","refresh_token":"refresh"}`))
	}))
	defer server.Close()

	t.Setenv("NEXUS_CONNECTOR_TWITTER_TOKEN_URL", server.URL)
	service := NewService(cfg, db)
	service.httpClient = server.Client()

	//goland:noinspection SqlResolve
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO connector_oauth_states (state, connector_id, code_verifier, redirect_uri, expires_at) VALUES (?, ?, ?, ?, datetime('now', '+10 minutes'))",
		"state-token",
		"x-twitter",
		"stored-verifier",
		cfg.ConnectorOAuthRedirectURI,
	)
	if err != nil {
		t.Fatalf("写入 OAuth state 失败: %v", err)
	}

	info, err := service.CompleteOAuthCallback(ctx, auth.SystemUserID, OAuthCallbackRequest{
		Code:        "code",
		State:       "state-token",
		RedirectURI: cfg.ConnectorOAuthRedirectURI,
	})
	if err != nil {
		t.Fatalf("完成 OAuth 回调失败: %v", err)
	}
	if info.ConnectionState != "connected" {
		t.Fatalf("连接状态未更新: %+v", info)
	}

	var remaining int
	//goland:noinspection SqlResolve
	if err = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM connector_oauth_states WHERE state = ?", "state-token").Scan(&remaining); err != nil {
		t.Fatalf("查询 OAuth state 失败: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("成功回调后 state 应删除: got=%d", remaining)
	}
}

func TestServiceOAuthCallbackConsumesStateBeforeTokenExchange(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "bad code", http.StatusBadRequest)
	}))
	defer server.Close()

	t.Setenv("NEXUS_CONNECTOR_TWITTER_TOKEN_URL", server.URL)
	service := NewService(cfg, db)
	service.httpClient = server.Client()

	//goland:noinspection SqlResolve
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO connector_oauth_states (state, connector_id, code_verifier, redirect_uri, expires_at) VALUES (?, ?, ?, ?, datetime('now', '+10 minutes'))",
		"state-token",
		"x-twitter",
		"stored-verifier",
		cfg.ConnectorOAuthRedirectURI,
	)
	if err != nil {
		t.Fatalf("写入 OAuth state 失败: %v", err)
	}

	_, err = service.CompleteOAuthCallback(ctx, auth.SystemUserID, OAuthCallbackRequest{
		Code:        "bad-code",
		State:       " state-token ",
		RedirectURI: cfg.ConnectorOAuthRedirectURI,
	})
	if err == nil {
		t.Fatal("token 交换失败时应返回错误")
	}

	var remaining int
	//goland:noinspection SqlResolve
	if err = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM connector_oauth_states WHERE state = ?", "state-token").Scan(&remaining); err != nil {
		t.Fatalf("查询 OAuth state 失败: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("token 交换失败后 state 也应已消费: got=%d", remaining)
	}
}

func TestServiceOAuthCallbackPassesStoredExtraJSONToProvider(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := NewService(cfg, db)
	service.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "demo.myshopify.com" {
			t.Fatalf("未使用 extra_json 里的 shop 构造 token URL: %s", request.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"shopify-token"}`)),
			Request:    request,
		}, nil
	})}

	//goland:noinspection SqlResolve
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO connector_oauth_states (state, connector_id, redirect_uri, extra_json, expires_at) VALUES (?, ?, ?, ?, datetime('now', '+10 minutes'))",
		"shopify-state",
		"shopify",
		cfg.ConnectorOAuthRedirectURI,
		`{"shop":"demo"}`,
	)
	if err != nil {
		t.Fatalf("写入 OAuth state 失败: %v", err)
	}

	info, err := service.CompleteOAuthCallback(ctx, auth.SystemUserID, OAuthCallbackRequest{
		Code:        "code",
		State:       "shopify-state",
		RedirectURI: cfg.ConnectorOAuthRedirectURI,
	})
	if err != nil {
		t.Fatalf("完成 Shopify OAuth 回调失败: %v", err)
	}
	if info.ConnectorID != "shopify" || info.ConnectionState != "connected" {
		t.Fatalf("连接状态未更新: %+v", info)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newConnectorsTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		Host:                         "127.0.0.1",
		Port:                         18013,
		ProjectName:                  "nexus-connectors-test",
		APIPrefix:                    "/nexus/v1",
		WebSocketPath:                "/nexus/v1/chat/ws",
		DefaultAgentID:               "nexus",
		WorkspacePath:                filepath.Join(root, "workspace"),
		CacheFileDir:                 filepath.Join(root, "cache"),
		DatabaseDriver:               "sqlite",
		DatabaseURL:                  filepath.Join(root, "nexus.db"),
		ConnectorOAuthRedirectURI:    "http://localhost:3000/capability/connectors/oauth/callback",
		ConnectorOAuthAllowedOrigins: []string{"http://localhost:3000"},
		ConnectorCredentialsKey:      "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		ConnectorGitHubClientID:      "github-client-id",
		ConnectorGitHubClientSecret:  "github-client-secret",
		ConnectorTwitterClientID:     "twitter-client-id",
		ConnectorTwitterClientSecret: "twitter-client-secret",
		ConnectorShopifyClientID:     "shopify-client-id",
		ConnectorShopifyClientSecret: "shopify-client-secret",
	}
}

func testConnectorCredentialKey() string {
	return "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
}

func migrateConnectorsSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, connectorsTestMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func connectorsTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
