package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	authstore "github.com/nexus-research-lab/nexus/internal/storage/auth"
)

// Login 执行密码登录并签发服务端 Session。
func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	state, err := s.GetState(ctx)
	if err != nil {
		return nil, err
	}
	if !state.PasswordLoginEnabled {
		return nil, ErrPasswordLoginDisabled
	}
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Password) == "" {
		return nil, errors.New("密码不能为空")
	}

	user, credential, err := s.repository.GetUserWithPasswordByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil || credential == nil || user.Status != UserStatusActive {
		return nil, ErrInvalidCredentials
	}
	matched, err := VerifyPassword(input.Password, credential.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ErrInvalidCredentials
	}

	now := s.now()
	sessionToken, err := s.tokenFactory()
	if err != nil {
		return nil, err
	}
	record := authstore.SessionRecord{
		SessionID:        s.idFactory("sess"),
		UserID:           user.UserID,
		SessionTokenHash: hashSessionToken(sessionToken),
		AuthMethod:       AuthMethodPassword,
		ExpiresAt:        now.Add(s.sessionTTL()),
		LastSeenAt:       now,
		ClientIP:         strings.TrimSpace(input.ClientIP),
		UserAgent:        strings.TrimSpace(input.UserAgent),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err = s.repository.CleanupExpiredSessions(ctx, now); err != nil {
		return nil, err
	}
	if err = s.repository.CreateSession(ctx, record); err != nil {
		return nil, err
	}
	if err = s.repository.UpdateUserLastLogin(ctx, user.UserID, now); err != nil {
		return nil, err
	}

	status := s.buildStatusPayload(state, &Principal{
		UserID:      user.UserID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Avatar:      user.Avatar,
		AuthMethod:  AuthMethodPassword,
		SessionID:   stringPointer(record.SessionID),
	})
	return &LoginResult{
		SessionToken: sessionToken,
		Status:       status,
	}, nil
}

// Logout 撤销当前浏览器 Session。
func (s *Service) Logout(ctx context.Context, sessionToken string) error {
	normalizedToken := strings.TrimSpace(sessionToken)
	if normalizedToken == "" {
		return nil
	}
	return s.repository.RevokeSessionByTokenHash(ctx, hashSessionToken(normalizedToken), s.now())
}

// ExtractSessionToken 从请求 Cookie 中提取服务端 Session。
func (s *Service) ExtractSessionToken(request *http.Request) string {
	if request == nil {
		return ""
	}
	cookie, err := request.Cookie(s.cookieName())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
