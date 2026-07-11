package connectors

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/service/auth"

	_ "modernc.org/sqlite"
)

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

func TestServiceOAuthCallbackWithoutRequestOwnerUsesStoredStateOwner(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.Header.Get("Content-Type"), "application/json") {
			t.Fatalf("飞书 token 请求应使用 JSON: %s", request.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("读取 token 请求失败: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"client_id":"owner-a-client"`) || !strings.Contains(text, `"client_secret":"owner-a-secret"`) {
			t.Fatalf("未使用 state owner 的 OAuth Client: %s", text)
		}
		_, _ = writer.Write([]byte(`{"code":0,"data":{"access_token":"owner-a-token","refresh_token":"refresh","expires_in":7200}}`))
	}))
	defer server.Close()

	t.Setenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL", server.URL)
	service := NewService(cfg, db)
	service.httpClient = server.Client()

	if _, err = service.SaveOAuthClientConfig(ctx, "owner-a", "feishu-docx", OAuthClientConfigRequest{
		ClientID:     "owner-a-client",
		ClientSecret: "owner-a-secret",
	}); err != nil {
		t.Fatalf("保存 owner-a OAuth Client 失败: %v", err)
	}
	authURL, err := service.GetAuthURL(ctx, "owner-a", "feishu-docx", "", nil)
	if err != nil {
		t.Fatalf("生成 OAuth 授权地址失败: %v", err)
	}

	info, err := service.CompleteOAuthCallback(ctx, "", OAuthCallbackRequest{
		Code:        "code",
		State:       authURL.State,
		RedirectURI: cfg.ConnectorOAuthRedirectURI,
	})
	if err != nil {
		t.Fatalf("完成无请求 owner 的 OAuth 回调失败: %v", err)
	}
	if info.ConnectionState != "connected" {
		t.Fatalf("连接状态未更新: %+v", info)
	}

	snapshot, err := service.LoadActiveConnection(ctx, "owner-a", "feishu-docx")
	if err != nil {
		t.Fatalf("读取 owner-a 连接失败: %v", err)
	}
	if snapshot == nil || snapshot.AccessToken != "owner-a-token" {
		t.Fatalf("连接未写入 state owner: %+v", snapshot)
	}
	systemSnapshot, err := service.LoadActiveConnection(ctx, auth.SystemUserID, "feishu-docx")
	if err != nil {
		t.Fatalf("读取 system 连接失败: %v", err)
	}
	if systemSnapshot != nil {
		t.Fatalf("无请求 owner 的回调不应写入 system owner: %+v", systemSnapshot)
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
