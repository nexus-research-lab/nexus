// INPUT: Connector OAuth state、回调参数、已上线与已移除产品标识。
// OUTPUT: 证明回调状态消费、owner 绑定、失败收口及未上线产品拒绝行为。
// POS: Connector OAuth 回调生命周期的服务端回归合同。
package connectors

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/service/auth"

	_ "modernc.org/sqlite"
)

func TestServiceOAuthCallbackUsesStoredState(t *testing.T) {
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
		if request.Form.Get("code") != "code" {
			t.Fatalf("未使用回调授权码: %v", request.Form)
		}
		_, _ = writer.Write([]byte(`{"access_token":"github-token","refresh_token":"refresh"}`))
	}))
	defer server.Close()

	t.Setenv("NEXUS_CONNECTOR_GITHUB_TOKEN_URL", server.URL)
	service := NewService(cfg, db)
	service.httpClient = server.Client()

	//goland:noinspection SqlResolve
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO connector_oauth_states (state, connector_id, redirect_uri, expires_at) VALUES (?, ?, ?, datetime('now', '+10 minutes'))",
		"state-token",
		"github",
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

	t.Setenv("NEXUS_CONNECTOR_GITHUB_TOKEN_URL", server.URL)
	service := NewService(cfg, db)
	service.httpClient = server.Client()

	//goland:noinspection SqlResolve
	_, err = db.ExecContext(
		ctx,
		"INSERT INTO connector_oauth_states (state, connector_id, code_verifier, redirect_uri, expires_at) VALUES (?, ?, ?, ?, datetime('now', '+10 minutes'))",
		"state-token",
		"github",
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

func TestServiceOAuthCallbackRejectsRemovedPlaceholder(t *testing.T) {
	cfg := newConnectorsTestConfig(t)
	migrateConnectorsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := NewService(cfg, db)
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

	_, err = service.CompleteOAuthCallback(ctx, auth.SystemUserID, OAuthCallbackRequest{
		Code:        "code",
		State:       "shopify-state",
		RedirectURI: cfg.ConnectorOAuthRedirectURI,
	})
	if err == nil || !strings.Contains(err.Error(), "未知连接器") {
		t.Fatalf("expected removed placeholder rejection, got %v", err)
	}
}
