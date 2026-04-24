package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestServiceSetupOwnerLoginLogoutAndResetPassword(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()

	state, err := service.GetState(ctx)
	if err != nil {
		t.Fatalf("读取初始状态失败: %v", err)
	}
	if !state.SetupRequired || state.AuthRequired || state.PasswordLoginEnabled {
		t.Fatalf("初始状态不正确: %+v", state)
	}

	user, err := service.InitOwner(ctx, InitOwnerInput{
		Username:    " Admin ",
		DisplayName: "系统管理员",
		Password:    "password123",
	})
	if err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}
	if user == nil || user.Role != RoleOwner || user.Username != "admin" {
		t.Fatalf("owner 数据不正确: %+v", user)
	}

	state, err = service.GetState(ctx)
	if err != nil {
		t.Fatalf("读取 owner 初始化后状态失败: %v", err)
	}
	if state.SetupRequired || !state.AuthRequired || !state.PasswordLoginEnabled || state.UserCount != 1 {
		t.Fatalf("owner 初始化后状态不正确: %+v", state)
	}

	if _, err = service.Login(ctx, LoginInput{
		Username: "admin",
		Password: "wrong-password",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("错误密码应返回 ErrInvalidCredentials，实际: %v", err)
	}

	loginResult, err := service.Login(ctx, LoginInput{
		Username:  "admin",
		Password:  "password123",
		ClientIP:  "127.0.0.1",
		UserAgent: "auth-test",
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if loginResult.SessionToken == "" {
		t.Fatal("登录未返回 session token")
	}
	if loginResult.Status.Username == nil || *loginResult.Status.Username != "admin" {
		t.Fatalf("登录状态未返回用户名: %+v", loginResult.Status)
	}
	if loginResult.Status.AuthMethod == nil || *loginResult.Status.AuthMethod != AuthMethodPassword {
		t.Fatalf("登录状态未返回密码认证方式: %+v", loginResult.Status)
	}

	request := httptest.NewRequest(http.MethodGet, "/agent/v1/auth/status", nil)
	request.AddCookie(&http.Cookie{
		Name:  service.CookieName(),
		Value: loginResult.SessionToken,
	})
	principal, inspectedState, err := service.InspectRequest(ctx, request)
	if err != nil {
		t.Fatalf("解析 cookie session 失败: %v", err)
	}
	if principal == nil || principal.Username != "admin" {
		t.Fatalf("cookie session 未解析出主体: %+v", principal)
	}
	if !inspectedState.AuthRequired {
		t.Fatalf("InspectRequest 返回的状态不正确: %+v", inspectedState)
	}

	users, err := service.ListUsers(ctx)
	if err != nil {
		t.Fatalf("列出用户失败: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("用户数量不正确: %d", len(users))
	}

	if _, err = service.ResetPassword(ctx, ResetPasswordInput{
		Username: "admin",
		Password: "password456",
	}); err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}
	if _, err = service.Login(ctx, LoginInput{
		Username: "admin",
		Password: "password123",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("旧密码应失效，实际错误: %v", err)
	}
	if _, err = service.Login(ctx, LoginInput{
		Username: "admin",
		Password: "password456",
	}); err != nil {
		t.Fatalf("新密码登录失败: %v", err)
	}

	if _, err = service.ChangePassword(ctx, ChangePasswordInput{
		UserID:          user.UserID,
		CurrentPassword: "password123",
		NewPassword:     "password789",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("个人改密应校验当前密码，实际: %v", err)
	}
	if _, err = service.ChangePassword(ctx, ChangePasswordInput{
		UserID:          user.UserID,
		CurrentPassword: "password456",
		NewPassword:     "password789",
	}); err != nil {
		t.Fatalf("个人改密失败: %v", err)
	}
	if _, err = service.Login(ctx, LoginInput{
		Username: "admin",
		Password: "password456",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("个人改密后旧密码应失效，实际: %v", err)
	}
	if _, err = service.Login(ctx, LoginInput{
		Username: "admin",
		Password: "password789",
	}); err != nil {
		t.Fatalf("个人改密后新密码登录失败: %v", err)
	}

	if err = service.Logout(ctx, loginResult.SessionToken); err != nil {
		t.Fatalf("登出失败: %v", err)
	}
	principal, _, err = service.InspectRequest(ctx, request)
	if err != nil {
		t.Fatalf("登出后解析请求失败: %v", err)
	}
	if principal != nil {
		t.Fatalf("登出后不应再解析出主体: %+v", principal)
	}
}

func TestServiceAccessTokenCompat(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	cfg.AccessToken = "compat-token"
	service := NewServiceWithDB(cfg, db)

	request := httptest.NewRequest(http.MethodGet, "/agent/v1/auth/status", nil)
	request.Header.Set("Authorization", "Bearer compat-token")

	principal, state, err := service.InspectRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("解析 ACCESS_TOKEN 兼容身份失败: %v", err)
	}
	if principal == nil || principal.AuthMethod != AuthMethodBearerCompat {
		t.Fatalf("兼容 bearer 身份不正确: %+v", principal)
	}
	if !state.AuthRequired || !state.AccessTokenEnabled {
		t.Fatalf("兼容 ACCESS_TOKEN 状态不正确: %+v", state)
	}
}

func TestServiceDisablesAccessTokenAfterOwnerInit(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	cfg.AccessToken = "compat-token"
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()

	if _, err := service.InitOwner(ctx, InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/agent/v1/auth/status", nil)
	request.Header.Set("Authorization", "Bearer compat-token")

	principal, state, err := service.InspectRequest(ctx, request)
	if err != nil {
		t.Fatalf("owner 初始化后解析 ACCESS_TOKEN 请求失败: %v", err)
	}
	if principal != nil {
		t.Fatalf("owner 初始化后不应再接受 ACCESS_TOKEN: %+v", principal)
	}
	if state.AccessTokenEnabled {
		t.Fatalf("owner 初始化后 access token 应关闭: %+v", state)
	}
	if !state.AuthRequired {
		t.Fatalf("owner 初始化后仍应要求认证: %+v", state)
	}
}

func newAuthTestDB(t *testing.T) (config.Config, *sql.DB) {
	t.Helper()

	root := t.TempDir()
	cfg := config.Config{
		APIPrefix:      "/agent/v1",
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "auth.db"),
	}

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开认证测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, authMigrationDir(t)); err != nil {
		t.Fatalf("执行 auth migration 失败: %v", err)
	}
	return cfg, db
}

func authMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 auth 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
