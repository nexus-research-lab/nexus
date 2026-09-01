package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	authstore "github.com/nexus-research-lab/nexus/internal/storage/auth"
)

var (
	// ErrUserNotFound 表示用户不存在。
	ErrUserNotFound = errors.New("user not found")
	// ErrPasswordLoginDisabled 表示系统未启用密码登录。
	ErrPasswordLoginDisabled = errors.New("服务端未启用密码登录")
	// ErrInvalidCredentials 表示用户名或密码无效。
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	// ErrPasswordChangeCommitted 表示密码凭据已提交，但后续响应投影失败。
	ErrPasswordChangeCommitted = errors.New("password change committed")
	// ErrPasswordChangeNotApplied 表示 exact request 已被 durable 终态收口为未执行。
	ErrPasswordChangeNotApplied = errors.New("password change not applied")
	// ErrPasswordChangeInvalidInput 表示密码修改协议字段校验失败。
	ErrPasswordChangeInvalidInput = errors.New("invalid password change input")
	// ErrOwnerAlreadyInitialized 表示 owner 已经创建，不能重复初始化。
	ErrOwnerAlreadyInitialized = errors.New("owner 用户已初始化")
	// ErrUsernameAlreadyExists 表示用户名已被占用。
	ErrUsernameAlreadyExists = errors.New("用户名已存在")
)

// Service 提供统一认证能力。
type Service struct {
	config            config.Config
	repository        *authstore.Repository
	now               func() time.Time
	idFactory         func(string) string
	tokenFactory      func() (string, error)
	runtimeTransition RuntimeTransitionCoordinator
	initOwnerMu       sync.Mutex
	humanSessionMu    sync.RWMutex
}

// NewServiceWithDB 使用共享 DB 创建认证服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		config:       cfg,
		repository:   authstore.NewRepository(cfg, db),
		now:          func() time.Time { return time.Now().UTC() },
		idFactory:    newAuthID,
		tokenFactory: newSessionToken,
	}
}

// GetState 返回认证系统状态。
func (s *Service) GetState(ctx context.Context) (State, error) {
	state, err := s.repository.LoadState(ctx, s.accessTokenEnabled())
	if err != nil {
		return State{}, err
	}
	result := toState(state)
	if s.desktopAuthBypassEnabled() {
		result.SetupRequired = false
		result.AuthRequired = false
		result.PasswordLoginEnabled = false
	}
	return result, nil
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

func toState(state authstore.State) State {
	return State{
		SetupRequired:        state.SetupRequired,
		AuthRequired:         state.AuthRequired,
		PasswordLoginEnabled: state.PasswordLoginEnabled,
		AccessTokenEnabled:   state.AccessTokenEnabled,
		UserCount:            state.UserCount,
		PasswordUserCount:    state.PasswordUserCount,
	}
}
