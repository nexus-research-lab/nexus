package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func TestAuthStatusLoginAndProtectedRoute(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	authService := authsvc.NewServiceWithDB(cfg, db)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
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

	protectedRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/nexus/v1/agents", nil)
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

func TestPersonalProfileAndChangePassword(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	authService := authsvc.NewServiceWithDB(cfg, db)
	if _, err := authService.InitOwner(context.Background(), authsvc.InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	cookie := loginByHTTP(t, httpServer.URL, "admin", "password123")
	profile := getPersonalProfile(t, httpServer.URL, cookie)
	if profile.User.Username != "admin" || !profile.CanChangePassword || !profile.CanUpdateProfile {
		t.Fatalf("个人设置资料不正确: %+v", profile)
	}
	if profile.User.Avatar != "" {
		t.Fatalf("初始头像应为空: %+v", profile.User)
	}
	if profile.TokenUsage.QuotaLimitTokens != nil || profile.TokenUsage.TotalTokens != 0 {
		t.Fatalf("初始 token 用量不正确: %+v", profile.TokenUsage)
	}
	updatedProfile := updatePersonalAvatar(t, httpServer.URL, cookie, "12")
	if updatedProfile.User.Avatar != "12" {
		t.Fatalf("头像更新未生效: %+v", updatedProfile.User)
	}
	statusWithAvatar := getAuthStatus(t, httpServer.URL, []*http.Cookie{cookie})
	if statusWithAvatar.Avatar == nil || *statusWithAvatar.Avatar != "12" {
		t.Fatalf("auth status 应返回最新头像: %+v", statusWithAvatar)
	}

	if status := changePasswordStatus(t, httpServer.URL, cookie, "wrong-password", "password456"); status != http.StatusUnprocessableEntity {
		t.Fatalf("错误当前密码应返回 422，实际: %d", status)
	}
	if status := changePasswordStatus(t, httpServer.URL, cookie, "password123", "password456"); status != http.StatusOK {
		t.Fatalf("正确当前密码应改密成功，实际: %d", status)
	}
	if status := loginStatus(t, httpServer.URL, "admin", "password123"); status != http.StatusUnauthorized {
		t.Fatalf("改密后旧密码应失败，实际: %d", status)
	}
	if status := loginStatus(t, httpServer.URL, "admin", "password456"); status != http.StatusOK {
		t.Fatalf("改密后新密码应成功，实际: %d", status)
	}
}

type authStatusResponse struct {
	AuthRequired         bool    `json:"auth_required"`
	PasswordLoginEnabled bool    `json:"password_login_enabled"`
	Authenticated        bool    `json:"authenticated"`
	Username             *string `json:"username"`
	Avatar               *string `json:"avatar"`
	SetupRequired        bool    `json:"setup_required"`
}

type personalProfileResponse struct {
	User struct {
		UserID      string `json:"user_id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Avatar      string `json:"avatar"`
		AuthMethod  string `json:"auth_method"`
	} `json:"user"`
	TokenUsage struct {
		TotalTokens      int64  `json:"total_tokens"`
		QuotaLimitTokens *int64 `json:"quota_limit_tokens"`
	} `json:"token_usage"`
	CanChangePassword bool `json:"can_change_password"`
	CanUpdateProfile  bool `json:"can_update_profile"`
}

type apiEnvelope[T any] struct {
	Data T `json:"data"`
}

func getAuthStatus(t *testing.T, baseURL string, cookies []*http.Cookie) authStatusResponse {
	t.Helper()

	request, _ := http.NewRequest(http.MethodGet, baseURL+"/nexus/v1/auth/status", nil)
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

	var payload apiEnvelope[authStatusResponse]
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("解析 auth status 响应失败: %v", err)
	}
	return payload.Data
}

func getPersonalProfile(t *testing.T, baseURL string, cookie *http.Cookie) personalProfileResponse {
	t.Helper()

	request, _ := http.NewRequest(http.MethodGet, baseURL+"/nexus/v1/settings/profile", nil)
	request.AddCookie(cookie)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("请求个人设置资料失败: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("个人设置资料状态码不正确: %d", response.StatusCode)
	}

	var payload apiEnvelope[personalProfileResponse]
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("解析个人设置资料失败: %v", err)
	}
	return payload.Data
}

func updatePersonalAvatar(t *testing.T, baseURL string, cookie *http.Cookie, avatar string) personalProfileResponse {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"avatar": avatar,
	})
	if err != nil {
		t.Fatalf("编码头像更新请求失败: %v", err)
	}

	request, _ := http.NewRequest(http.MethodPatch, baseURL+"/nexus/v1/settings/profile", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("头像更新请求失败: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("头像更新状态码不正确: %d", response.StatusCode)
	}

	var payload apiEnvelope[personalProfileResponse]
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("解析头像更新响应失败: %v", err)
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

	request, _ := http.NewRequest(http.MethodPost, baseURL+"/nexus/v1/auth/login", bytes.NewReader(body))
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

func loginStatus(t *testing.T, baseURL string, username string, password string) int {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		t.Fatalf("编码登录请求失败: %v", err)
	}
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/nexus/v1/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("登录请求失败: %v", err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func changePasswordStatus(
	t *testing.T,
	baseURL string,
	cookie *http.Cookie,
	currentPassword string,
	newPassword string,
) int {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"current_password": currentPassword,
		"new_password":     newPassword,
	})
	if err != nil {
		t.Fatalf("编码改密请求失败: %v", err)
	}
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/nexus/v1/settings/profile/password", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("改密请求失败: %v", err)
	}
	defer response.Body.Close()
	return response.StatusCode
}
