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
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"

	_ "modernc.org/sqlite"
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

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil)
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
		RequestID:       "test-password:invalid-current",
		CurrentPassword: "password123",
		NewPassword:     "password789",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("个人改密应校验当前密码，实际: %v", err)
	}
	if outcome, outcomeErr := service.PasswordChangeOutcome(
		ctx,
		user.UserID,
		"test-password:invalid-current",
	); outcomeErr != nil || outcome != PasswordChangeOutcomeNotApplied {
		t.Fatalf("明确拒绝必须留下 not_applied 回执: outcome=%q err=%v", outcome, outcomeErr)
	}
	if _, err = service.ChangePassword(ctx, ChangePasswordInput{
		UserID:          user.UserID,
		RequestID:       "test-password:invalid-current",
		CurrentPassword: "password456",
		NewPassword:     "password789",
	}); !errors.Is(err, ErrPasswordChangeNotApplied) {
		t.Fatalf("not_applied request 不得以新输入复用: %v", err)
	}
	if _, err = service.ChangePassword(ctx, ChangePasswordInput{
		UserID:          user.UserID,
		RequestID:       "test-password:committed",
		CurrentPassword: "password456",
		NewPassword:     "password789",
	}); err != nil {
		t.Fatalf("个人改密失败: %v", err)
	}
	if _, err = service.ChangePassword(ctx, ChangePasswordInput{
		UserID:          user.UserID,
		RequestID:       "test-password:committed",
		CurrentPassword: "password456",
		NewPassword:     "must-not-replace-password789",
	}); err != nil {
		t.Fatalf("同一 exact request 应返回已提交回执而不是再次改密: %v", err)
	}
	outcome, err := service.PasswordChangeOutcome(ctx, user.UserID, "test-password:committed")
	if err != nil || outcome != PasswordChangeOutcomeCommitted {
		t.Fatalf("改密回执应与凭据同事务提交: outcome=%q err=%v", outcome, err)
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

func TestServiceAccessTokenBearer(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	cfg.AccessToken = "access-token"
	service := NewServiceWithDB(cfg, db)

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil)
	request.Header.Set("Authorization", "Bearer access-token")

	principal, state, err := service.InspectRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("解析 ACCESS_TOKEN bearer 身份失败: %v", err)
	}
	if principal == nil || principal.AuthMethod != AuthMethodBearer {
		t.Fatalf("bearer 身份不正确: %+v", principal)
	}
	if !state.AuthRequired || !state.AccessTokenEnabled {
		t.Fatalf("ACCESS_TOKEN 状态不正确: %+v", state)
	}
}

func TestVerifyInteractiveHumanRequiresLivePasswordSession(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()
	owner, err := service.InitOwner(ctx, InitOwnerInput{
		Username: "admin",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(ctx, LoginInput{
		Username: "admin",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil)
	request.AddCookie(&http.Cookie{Name: service.CookieName(), Value: login.SessionToken})
	principal, _, err := service.InspectRequest(ctx, request)
	if err != nil || principal == nil || principal.SessionID == nil {
		t.Fatalf("resolve password principal: principal=%+v err=%v", principal, err)
	}
	verified, err := service.VerifyInteractiveHuman(ctx, principal)
	if err != nil || verified == nil || verified.UserID != owner.UserID ||
		verified.Role != RoleOwner || verified.AuthMethod != AuthMethodPassword {
		t.Fatalf("verify active password human: principal=%+v err=%v", verified, err)
	}
	if role, roleErr := service.ResolveActivePrincipalRole(ctx, owner.UserID); roleErr != nil ||
		role != RoleOwner {
		t.Fatalf("resolve active principal role: role=%q err=%v", role, roleErr)
	}
	if err = service.Logout(ctx, login.SessionToken); err != nil {
		t.Fatal(err)
	}
	if _, err = service.VerifyInteractiveHuman(ctx, principal); err == nil {
		t.Fatal("revoked password session approved a destructive configuration change")
	}
	if _, err = service.VerifyInteractiveHuman(ctx, &Principal{
		UserID: owner.UserID, Role: RoleOwner, AuthMethod: AuthMethodBearer,
	}); err == nil {
		t.Fatal("bearer principal approved a destructive configuration change")
	}
}

func TestBoundInteractiveHumanLeaseSerializesLogout(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()
	owner, err := service.InitOwner(ctx, InitOwnerInput{
		Username: "admin",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(ctx, LoginInput{
		Username: "admin",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil)
	request.AddCookie(&http.Cookie{Name: service.CookieName(), Value: login.SessionToken})
	principal, _, err := service.InspectRequest(ctx, request)
	if err != nil || principal == nil || principal.SessionID == nil {
		t.Fatalf("resolve password principal: principal=%+v err=%v", principal, err)
	}
	verified, release, err := service.AcquireBoundInteractiveHumanLease(
		ctx,
		owner.UserID,
		AuthMethodPassword,
		*principal.SessionID,
	)
	if err != nil || verified == nil || release == nil {
		t.Fatalf("acquire human lease: principal=%+v release=%v err=%v", verified, release != nil, err)
	}

	logoutDone := make(chan error, 1)
	go func() {
		logoutDone <- service.Logout(ctx, login.SessionToken)
	}()
	select {
	case logoutErr := <-logoutDone:
		t.Fatalf("logout crossed active human lease: %v", logoutErr)
	case <-time.After(50 * time.Millisecond):
	}
	release()
	select {
	case logoutErr := <-logoutDone:
		if logoutErr != nil {
			t.Fatalf("logout after release: %v", logoutErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("logout did not resume after human lease release")
	}
	if _, err = service.VerifyBoundInteractiveHuman(
		ctx,
		owner.UserID,
		AuthMethodPassword,
		*principal.SessionID,
	); err == nil {
		t.Fatal("released and revoked password session remained valid")
	}
}

func TestVerifyInteractiveHumanRequiresDesktopSessionEvidence(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	cfg.AppMode = "desktop"
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()
	principal, _, err := service.InspectRequest(
		ctx,
		httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil),
	)
	if err != nil || principal == nil || principal.AuthMethod != AuthMethodLocal {
		t.Fatalf("resolve desktop principal: principal=%+v err=%v", principal, err)
	}
	if _, err = service.VerifyInteractiveHuman(ctx, principal); err == nil {
		t.Fatal("desktop local principal without session token evidence was accepted")
	}
	wrongEvidence := authctx.WithInteractiveHumanEvidence(ctx, "forged")
	if _, err = service.VerifyInteractiveHuman(wrongEvidence, principal); err == nil {
		t.Fatal("unknown desktop evidence was accepted")
	}
	verified, err := service.VerifyInteractiveHuman(
		authctx.WithInteractiveHumanEvidence(ctx, "desktop_session_token"),
		principal,
	)
	if err != nil || verified == nil || verified.UserID != SystemUserID ||
		verified.Role != RoleOwner {
		t.Fatalf("verify desktop human: principal=%+v err=%v", verified, err)
	}
}

func TestServiceDesktopModeBypassesPasswordAuth(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	cfg.AppMode = "desktop"
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()

	if _, err := service.InitOwner(ctx, InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}

	state, err := service.GetState(ctx)
	if err != nil {
		t.Fatalf("读取 desktop auth 状态失败: %v", err)
	}
	if state.SetupRequired || state.AuthRequired || state.PasswordLoginEnabled {
		t.Fatalf("desktop 模式不应要求本地账号登录: %+v", state)
	}

	status, err := service.BuildStatusPayload(ctx, httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil))
	if err != nil {
		t.Fatalf("构建 desktop auth 状态失败: %v", err)
	}
	if status.AuthRequired || status.SetupRequired || !status.Authenticated {
		t.Fatalf("desktop auth status 不正确: %+v", status)
	}
	if status.Username == nil || *status.Username != "local" {
		t.Fatalf("desktop auth status 应返回本地用户: %+v", status)
	}
	if status.AuthMethod == nil || *status.AuthMethod != AuthMethodLocal {
		t.Fatalf("desktop auth status 应返回本地认证方式: %+v", status)
	}

	avatar := "12"
	user, err := service.UpdateProfile(ctx, UpdateProfileInput{
		UserID: SystemUserID,
		Avatar: &avatar,
	})
	if err != nil {
		t.Fatalf("更新 desktop 本地头像失败: %v", err)
	}
	if user.Avatar != avatar {
		t.Fatalf("desktop 本地头像未持久化: %+v", user)
	}

	status, err = service.BuildStatusPayload(ctx, httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil))
	if err != nil {
		t.Fatalf("构建带头像的 desktop auth 状态失败: %v", err)
	}
	if status.Avatar == nil || *status.Avatar != avatar {
		t.Fatalf("desktop auth status 应返回本地头像: %+v", status)
	}

	serverCfg := cfg
	serverCfg.AppMode = ""
	serverService := NewServiceWithDB(serverCfg, db)
	serverState, err := serverService.GetState(ctx)
	if err != nil {
		t.Fatalf("读取 server auth 状态失败: %v", err)
	}
	if serverState.UserCount != 1 || serverState.PasswordUserCount != 1 {
		t.Fatalf("本地桌面用户不应计入服务端账号状态: %+v", serverState)
	}
}

func TestServiceDisablesAccessTokenAfterOwnerInit(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	cfg.AccessToken = "access-token"
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()

	if _, err := service.InitOwner(ctx, InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/auth/status", nil)
	request.Header.Set("Authorization", "Bearer access-token")

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
		APIPrefix:      "/nexus/v1",
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "auth.db"),
	}

	handlertest.MigrateSQLiteFromDir(t, cfg.DatabaseURL, authMigrationDir(t))
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开认证测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return cfg, db
}

func authMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 auth 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
