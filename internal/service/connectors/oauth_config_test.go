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

	"github.com/nexus-research-lab/nexus/internal/service/auth"

	_ "modernc.org/sqlite"
)

func TestServiceScopesOAuthStateByOwner(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	service := NewService(cfg, db)
	ctx := context.Background()
	authA, err := service.GetAuthURL(ctx, "owner-a", "github", "", nil)
	if err != nil {
		t.Fatalf("生成 owner-a 授权地址失败: %v", err)
	}
	authB, err := service.GetAuthURL(ctx, "owner-b", "github", "", nil)
	if err != nil {
		t.Fatalf("生成 owner-b 授权地址失败: %v", err)
	}
	if authA.State == authB.State {
		t.Fatalf("两次 state 不应相同: %q", authA.State)
	}

	var ownerAStateCount int
	//goland:noinspection SqlResolve
	if err = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM connector_oauth_states WHERE owner_user_id = ? AND state = ?", "owner-a", authA.State).Scan(&ownerAStateCount); err != nil {
		t.Fatalf("查询 owner-a OAuth state 失败: %v", err)
	}
	if ownerAStateCount != 1 {
		t.Fatalf("owner-a OAuth state 应按 owner 落库: got=%d want=1", ownerAStateCount)
	}

	_, err = service.CompleteOAuthCallback(ctx, "owner-b", OAuthCallbackRequest{
		Code:  "wrong-owner-code",
		State: authA.State,
	})
	if err == nil || !strings.Contains(err.Error(), "OAuth state 无效") {
		t.Fatalf("owner-b 不应消费 owner-a OAuth state: %v", err)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM connector_oauth_states WHERE owner_user_id = ? AND state = ?", "owner-a", authA.State).Scan(&ownerAStateCount); err != nil {
		t.Fatalf("再次查询 owner-a OAuth state 失败: %v", err)
	}
	if ownerAStateCount != 1 {
		t.Fatalf("跨 owner callback 不应删除 owner-a OAuth state: got=%d want=1", ownerAStateCount)
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
	if len(items) != 1 || items[0].IsConfigured || !items[0].OAuthClientConfigRequired {
		t.Fatalf("未保存用户 OAuth Client 前应为待配置: %+v", items)
	}
	if items[0].ConfigError == nil || !strings.Contains(*items[0].ConfigError, "自己的 OAuth 应用") {
		t.Fatalf("配置错误应提示用户配置自己的 OAuth 应用: %+v", items[0].ConfigError)
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
	if !info.IsConfigured || !info.OAuthClientConfigured {
		t.Fatalf("保存后应视为已配置: %+v", info)
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
