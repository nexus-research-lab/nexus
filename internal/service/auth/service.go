package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
)

var (
	// ErrPasswordLoginDisabled 表示系统未启用密码登录。
	ErrPasswordLoginDisabled = errors.New("服务端未启用密码登录")
	// ErrInvalidCredentials 表示用户名或密码无效。
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	// ErrOwnerAlreadyInitialized 表示 owner 已经创建，不能重复初始化。
	ErrOwnerAlreadyInitialized = errors.New("owner 用户已初始化")
	// ErrUsernameAlreadyExists 表示用户名已被占用。
	ErrUsernameAlreadyExists = errors.New("用户名已存在")
)

// Service 提供统一认证能力。
type Service struct {
	config       config.Config
	repository   *repository
	now          func() time.Time
	idFactory    func(string) string
	tokenFactory func() (string, error)
}

// NewServiceWithDB 使用共享 DB 创建认证服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		config:       cfg,
		repository:   newRepository(cfg, db),
		now:          func() time.Time { return time.Now().UTC() },
		idFactory:    newAuthID,
		tokenFactory: newSessionToken,
	}
}

// GetState 返回认证系统状态。
func (s *Service) GetState(ctx context.Context) (State, error) {
	return s.repository.loadState(ctx, s.accessTokenEnabled())
}

// InspectRequest 解析请求身份并返回认证系统状态。
func (s *Service) InspectRequest(ctx context.Context, request *http.Request) (*Principal, State, error) {
	state, err := s.GetState(ctx)
	if err != nil {
		return nil, State{}, err
	}
	principal, err := s.resolveRequestPrincipal(ctx, request, state)
	if err != nil {
		return nil, state, err
	}
	return principal, state, nil
}

// BuildStatusPayload 构建当前请求可消费的认证状态。
func (s *Service) BuildStatusPayload(ctx context.Context, request *http.Request) (StatusPayload, error) {
	principal, state, err := s.InspectRequest(ctx, request)
	if err != nil {
		return StatusPayload{}, err
	}
	return s.buildStatusPayload(state, principal), nil
}

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

	user, credential, err := s.repository.getUserWithPasswordByUsername(ctx, username)
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
	record := sessionRecord{
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
	if err = s.repository.cleanupExpiredSessions(ctx, now); err != nil {
		return nil, err
	}
	if err = s.repository.createSession(ctx, record); err != nil {
		return nil, err
	}
	if err = s.repository.updateUserLastLogin(ctx, user.UserID, now); err != nil {
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
	return s.repository.revokeSessionByTokenHash(ctx, hashSessionToken(normalizedToken), s.now())
}

// InitOwner 初始化第一个 owner 用户。
func (s *Service) InitOwner(ctx context.Context, input InitOwnerInput) (*User, error) {
	state, err := s.GetState(ctx)
	if err != nil {
		return nil, err
	}
	if state.UserCount > 0 {
		return nil, ErrOwnerAlreadyInitialized
	}

	username, err := normalizeUsername(input.Username)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if err = validatePassword(input.Password); err != nil {
		return nil, err
	}
	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	now := s.now()
	user := User{
		UserID:      s.idFactory("user"),
		Username:    username,
		DisplayName: displayName,
		Role:        RoleOwner,
		Status:      UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	credential := passwordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = s.repository.createUserWithPassword(ctx, user, credential); err != nil {
		return nil, err
	}
	return s.repository.getUserByID(ctx, user.UserID)
}

// CreateUser 创建新的认证用户。
func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if err = validatePassword(input.Password); err != nil {
		return nil, err
	}
	role, err := normalizeUserRole(input.Role)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameAlreadyExists
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	now := s.now()
	user := User{
		UserID:      s.idFactory("user"),
		Username:    username,
		DisplayName: displayName,
		Role:        role,
		Status:      UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	credential := passwordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = s.repository.createUserWithPassword(ctx, user, credential); err != nil {
		return nil, err
	}
	return s.repository.getUserByID(ctx, user.UserID)
}

// ListUsers 列出当前全部用户。
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.repository.listUsers(ctx)
}

// ResetPassword 重置指定用户密码。
func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) (*User, error) {
	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}

	var (
		user *User
		err  error
	)
	if strings.TrimSpace(input.UserID) != "" {
		user, err = s.repository.getUserByID(ctx, input.UserID)
	} else if strings.TrimSpace(input.Username) != "" {
		user, err = s.repository.getUserByUsername(ctx, strings.TrimSpace(input.Username))
	} else {
		return nil, errors.New("user_id 与 username 至少提供一个")
	}
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	now := s.now()
	credential := passwordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = s.repository.upsertPasswordCredential(ctx, credential); err != nil {
		return nil, err
	}
	return s.repository.getUserByID(ctx, user.UserID)
}

// ChangePassword 校验当前密码后修改当前用户密码。
func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) (*User, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("user_id 不能为空")
	}
	if strings.TrimSpace(input.CurrentPassword) == "" {
		return nil, errors.New("当前密码不能为空")
	}
	if err := validatePassword(input.NewPassword); err != nil {
		return nil, err
	}

	user, credential, err := s.repository.getUserWithPasswordByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || credential == nil || user.Status != UserStatusActive {
		return nil, ErrInvalidCredentials
	}
	matched, err := VerifyPassword(input.CurrentPassword, credential.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ErrInvalidCredentials
	}

	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return nil, err
	}
	now := s.now()
	nextCredential := passwordCredential{
		CredentialID:      s.idFactory("cred"),
		UserID:            user.UserID,
		PasswordHash:      passwordHash,
		PasswordAlgo:      passwordAlgorithmArgon2ID,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = s.repository.upsertPasswordCredential(ctx, nextCredential); err != nil {
		return nil, err
	}
	return s.repository.getUserByID(ctx, user.UserID)
}

// UpdateProfile 更新当前用户的个人资料。
func (s *Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (*User, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return nil, errors.New("user_id 不能为空")
	}

	user, err := s.repository.getUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != UserStatusActive {
		return nil, ErrUserNotFound
	}

	if input.Avatar != nil {
		avatar, avatarErr := normalizeAvatar(*input.Avatar)
		if avatarErr != nil {
			return nil, avatarErr
		}
		if err = s.repository.updateUserAvatar(ctx, userID, avatar, s.now()); err != nil {
			return nil, err
		}
	}
	return s.repository.getUserByID(ctx, userID)
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

// CookieName 返回认证 Cookie 名称。
func (s *Service) CookieName() string {
	return s.cookieName()
}

// CookiePath 返回认证 Cookie 作用路径。
func (s *Service) CookiePath() string {
	return s.cookiePath()
}

// CookieSecure 返回认证 Cookie 的 secure 配置。
func (s *Service) CookieSecure() bool {
	return s.config.AuthCookieSecure
}

// CookieSameSite 返回认证 Cookie 的 SameSite 配置。
func (s *Service) CookieSameSite() http.SameSite {
	switch strings.ToLower(strings.TrimSpace(s.config.AuthCookieSameSite)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// SessionMaxAge 返回认证 Cookie 过期秒数。
func (s *Service) SessionMaxAge() int {
	return int(s.sessionTTL().Seconds())
}

func (s *Service) buildStatusPayload(state State, principal *Principal) StatusPayload {
	result := StatusPayload{
		AuthRequired:         state.AuthRequired,
		PasswordLoginEnabled: state.PasswordLoginEnabled,
		Authenticated:        !state.AuthRequired && !state.SetupRequired,
		SetupRequired:        state.SetupRequired,
		AccessTokenEnabled:   state.AccessTokenEnabled,
	}
	if principal == nil {
		return result
	}
	result.Authenticated = true
	result.Username = stringPointer(principal.Username)
	result.UserID = stringPointer(principal.UserID)
	result.DisplayName = stringPointer(principal.DisplayName)
	result.Role = stringPointer(principal.Role)
	result.Avatar = stringPointer(principal.Avatar)
	result.AuthMethod = stringPointer(principal.AuthMethod)
	return result
}

func (s *Service) resolveRequestPrincipal(ctx context.Context, request *http.Request, state State) (*Principal, error) {
	if request == nil {
		return nil, nil
	}
	sessionToken := s.ExtractSessionToken(request)
	if sessionToken != "" {
		principal, err := s.resolveSessionPrincipal(ctx, sessionToken)
		if err != nil {
			return nil, err
		}
		if principal != nil {
			return principal, nil
		}
	}
	if !state.AccessTokenEnabled {
		return nil, nil
	}
	return s.resolveBearerPrincipal(request), nil
}

func (s *Service) resolveSessionPrincipal(ctx context.Context, sessionToken string) (*Principal, error) {
	record, user, err := s.repository.getActiveSessionByTokenHash(ctx, hashSessionToken(sessionToken), s.now())
	if err != nil {
		return nil, err
	}
	if record == nil || user == nil || user.Status != UserStatusActive {
		return nil, nil
	}
	if err = s.repository.touchSession(ctx, record.SessionID, s.now()); err != nil {
		return nil, err
	}
	return &Principal{
		UserID:      user.UserID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Avatar:      user.Avatar,
		AuthMethod:  AuthMethodPassword,
		SessionID:   stringPointer(record.SessionID),
	}, nil
}

func (s *Service) resolveBearerPrincipal(request *http.Request) *Principal {
	accessToken := strings.TrimSpace(s.config.AccessToken)
	if accessToken == "" || request == nil {
		return nil
	}
	providedToken := extractBearerToken(request.Header.Get("Authorization"))
	if providedToken == "" {
		providedToken = firstNonEmpty(
			request.URL.Query().Get("access_token"),
			request.URL.Query().Get("token"),
		)
	}
	if providedToken == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(accessToken)) != 1 {
		return nil
	}
	return &Principal{
		UserID:      "legacy-access-token",
		Username:    "access-token",
		DisplayName: "ACCESS_TOKEN",
		Role:        RoleOwner,
		AuthMethod:  AuthMethodBearer,
	}
}

func (s *Service) accessTokenEnabled() bool {
	return strings.TrimSpace(s.config.AccessToken) != ""
}

func normalizeUserRole(role string) (string, error) {
	switch strings.TrimSpace(role) {
	case "", RoleMember:
		return RoleMember, nil
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleOwner:
		return RoleOwner, nil
	default:
		return "", errors.New("role 仅支持 owner、admin、member")
	}
}

func (s *Service) sessionTTL() time.Duration {
	hours := s.config.AuthSessionTTLHours
	if hours <= 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

func (s *Service) cookieName() string {
	name := strings.TrimSpace(s.config.AuthSessionCookieName)
	if name == "" {
		return "nexus_session"
	}
	return name
}

func (s *Service) cookiePath() string {
	path := strings.TrimSpace(s.config.APIPrefix)
	if path == "" {
		return "/"
	}
	return path
}

func normalizeUsername(username string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(username))
	if normalized == "" {
		return "", errors.New("用户名不能为空")
	}
	if len(normalized) < 3 || len(normalized) > 64 {
		return "", errors.New("用户名长度必须在 3 到 64 个字符之间")
	}
	for _, item := range normalized {
		if (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9') || item == '-' || item == '_' || item == '.' {
			continue
		}
		return "", errors.New("用户名只能包含小写字母、数字、点、横线和下划线")
	}
	return normalized, nil
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("密码不能为空")
	}
	if len(password) < 8 {
		return errors.New("密码长度至少需要 8 个字符")
	}
	return nil
}

func normalizeAvatar(avatar string) (string, error) {
	normalized := strings.TrimSpace(avatar)
	if len(normalized) > 255 {
		return "", errors.New("头像标识不能超过 255 个字符")
	}
	return normalized, nil
}

func extractBearerToken(rawAuthorization string) string {
	header := strings.TrimSpace(rawAuthorization)
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func hashSessionToken(sessionToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sessionToken)))
	return hex.EncodeToString(sum[:])
}

func newSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func newAuthID(prefix string) string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UTC().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}

func stringPointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// ResolveClientIP 尝试从请求中提取真实客户端 IP。
func ResolveClientIP(request *http.Request) string {
	if request == nil {
		return ""
	}
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := strings.TrimSpace(request.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(request.RemoteAddr)
}
