// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：auth_handlers_test.go
// @Date   ：2026/04/12 10:42:00
// @Author ：leemysw
// 2026/04/12 10:42:00   Create
// =====================================================

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"

	"github.com/coder/websocket"
)

func TestAuthStatusLoginAndProtectedRoute(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	initialStatus := getAuthStatus(t, httpServer.URL, nil)
	if !initialStatus.SetupRequired || initialStatus.AuthRequired {
		t.Fatalf("初始 auth 状态不正确: %+v", initialStatus)
	}

	if _, err = server.auth.InitOwner(context.Background(), authsvc.InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}

	protectedRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/agent/v1/agents", nil)
	protectedResponse, err := http.DefaultClient.Do(protectedRequest)
	if err != nil {
		t.Fatalf("请求受保护路由失败: %v", err)
	}
	defer protectedResponse.Body.Close()
	if protectedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未登录访问受保护路由应返回 401，实际: %d", protectedResponse.StatusCode)
	}

	statusAfterInit := getAuthStatus(t, httpServer.URL, nil)
	if !statusAfterInit.AuthRequired || statusAfterInit.Authenticated {
		t.Fatalf("owner 初始化后状态不正确: %+v", statusAfterInit)
	}

	cookie := loginByHTTP(t, httpServer.URL, "admin", "password123")
	if cookie == nil || strings.TrimSpace(cookie.Value) == "" {
		t.Fatal("登录未返回有效 cookie")
	}

	statusAfterLogin := getAuthStatus(t, httpServer.URL, []*http.Cookie{cookie})
	if !statusAfterLogin.Authenticated || statusAfterLogin.Username == nil || *statusAfterLogin.Username != "admin" {
		t.Fatalf("登录后的 auth 状态不正确: %+v", statusAfterLogin)
	}

	authorizedRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/agent/v1/agents", nil)
	authorizedRequest.AddCookie(cookie)
	authorizedResponse, err := http.DefaultClient.Do(authorizedRequest)
	if err != nil {
		t.Fatalf("携带 cookie 请求受保护路由失败: %v", err)
	}
	defer authorizedResponse.Body.Close()
	if authorizedResponse.StatusCode != http.StatusOK {
		t.Fatalf("携带 cookie 请求受保护路由应返回 200，实际: %d", authorizedResponse.StatusCode)
	}

	logoutRequest, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/agent/v1/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	logoutResponse, err := http.DefaultClient.Do(logoutRequest)
	if err != nil {
		t.Fatalf("登出请求失败: %v", err)
	}
	defer logoutResponse.Body.Close()
	if logoutResponse.StatusCode != http.StatusOK {
		t.Fatalf("登出请求应返回 200，实际: %d", logoutResponse.StatusCode)
	}

	expiredRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/agent/v1/agents", nil)
	expiredRequest.AddCookie(cookie)
	expiredResponse, err := http.DefaultClient.Do(expiredRequest)
	if err != nil {
		t.Fatalf("验证过期 cookie 请求失败: %v", err)
	}
	defer expiredResponse.Body.Close()
	if expiredResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("登出后旧 cookie 应失效，实际状态码: %d", expiredResponse.StatusCode)
	}
}

func TestAuthBearerCompatAndWebSocketHandshake(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	cfg.AccessToken = "compat-token"
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	request, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/agent/v1/agents", nil)
	request.Header.Set("Authorization", "Bearer compat-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("兼容 bearer 访问受保护路由失败: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("兼容 bearer 访问受保护路由应返回 200，实际: %d", response.StatusCode)
	}

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/agent/v1/chat/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	unauthorizedConn, unauthorizedResponse, err := websocket.Dial(ctx, wsURL, nil)
	if unauthorizedConn != nil {
		_ = unauthorizedConn.Close(websocket.StatusNormalClosure, "unexpected")
	}
	if err == nil {
		t.Fatal("未携带认证信息的 WebSocket 握手应失败")
	}
	if unauthorizedResponse == nil || unauthorizedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未授权 WebSocket 握手状态码不正确: %+v", unauthorizedResponse)
	}

	authorizedConn, _, err := websocket.Dial(ctx, wsURL+"?access_token=compat-token", nil)
	if err != nil {
		t.Fatalf("携带兼容 access_token 的 WebSocket 握手失败: %v", err)
	}
	_ = authorizedConn.Close(websocket.StatusNormalClosure, "test done")
}

func TestAuthDisablesBearerCompatAfterOwnerInit(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	cfg.AccessToken = "compat-token"
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}
	if _, err = server.auth.InitOwner(context.Background(), authsvc.InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}

	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	request, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/agent/v1/agents", nil)
	request.Header.Set("Authorization", "Bearer compat-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("owner 初始化后 bearer 请求失败: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("owner 初始化后 bearer compat 应返回 401，实际: %d", response.StatusCode)
	}

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/agent/v1/chat/ws?access_token=compat-token"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, handshake, err := websocket.Dial(ctx, wsURL, nil)
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "unexpected")
	}
	if err == nil {
		t.Fatal("owner 初始化后携带 access_token 的 WebSocket 握手应失败")
	}
	if handshake == nil || handshake.StatusCode != http.StatusUnauthorized {
		t.Fatalf("owner 初始化后 WebSocket 握手状态码不正确: %+v", handshake)
	}
}

type authStatusResponse struct {
	AuthRequired         bool    `json:"auth_required"`
	PasswordLoginEnabled bool    `json:"password_login_enabled"`
	Authenticated        bool    `json:"authenticated"`
	Username             *string `json:"username"`
	SetupRequired        bool    `json:"setup_required"`
}

type gatewayEnvelope[T any] struct {
	Data T `json:"data"`
}

func getAuthStatus(t *testing.T, baseURL string, cookies []*http.Cookie) authStatusResponse {
	t.Helper()

	request, _ := http.NewRequest(http.MethodGet, baseURL+"/agent/v1/auth/status", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("请求 auth status 失败: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("auth status 状态码不正确: %d", response.StatusCode)
	}

	var payload gatewayEnvelope[authStatusResponse]
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("解析 auth status 响应失败: %v", err)
	}
	return payload.Data
}

func loginByHTTP(t *testing.T, baseURL string, username string, password string) *http.Cookie {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		t.Fatalf("编码登录请求失败: %v", err)
	}

	request, _ := http.NewRequest(http.MethodPost, baseURL+"/agent/v1/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("登录请求失败: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("登录状态码不正确: %d", response.StatusCode)
	}
	for _, cookie := range response.Cookies() {
		if strings.TrimSpace(cookie.Name) != "" {
			return cookie
		}
	}
	t.Fatal("登录响应未返回 cookie")
	return nil
}
