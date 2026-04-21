package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	"github.com/nexus-research-lab/nexus/internal/gateway"
	"github.com/nexus-research-lab/nexus/internal/gateway/gatewaytest"
)

func TestAuthStatusLoginAndProtectedRoute(t *testing.T) {
	cfg := gatewaytest.NewConfig(t)
	gatewaytest.MigrateSQLite(t, cfg.DatabaseURL)

	db := gatewaytest.OpenSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	authService := authsvc.NewServiceWithDB(cfg, db)

	server, err := gateway.NewServer(cfg)
	if err != nil {
		t.Fatalf("创建 gateway 失败: %v", err)
	}
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	initialStatus := getAuthStatus(t, httpServer.URL, nil)
	if !initialStatus.SetupRequired || initialStatus.AuthRequired {
		t.Fatalf("初始 auth 状态不正确: %+v", initialStatus)
	}

	if _, err = authService.InitOwner(context.Background(), authsvc.InitOwnerInput{
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

	cookie := loginByHTTP(t, httpServer.URL, "admin", "password123")
	if cookie == nil || strings.TrimSpace(cookie.Value) == "" {
		t.Fatal("登录未返回有效 cookie")
	}

	statusAfterLogin := getAuthStatus(t, httpServer.URL, []*http.Cookie{cookie})
	if !statusAfterLogin.Authenticated || statusAfterLogin.Username == nil || *statusAfterLogin.Username != "admin" {
		t.Fatalf("登录后的 auth 状态不正确: %+v", statusAfterLogin)
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
