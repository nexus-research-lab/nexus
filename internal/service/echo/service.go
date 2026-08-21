// INPUT: Echo attempt 仓储、用户 Preferences、Agent/Session/DM 服务、轻量模型与 runtime 状态。
// OUTPUT: 用户级开关、DM 覆盖、用户活动取消与后台调度生命周期。
// POS: Echo 应用服务装配根。
package echo

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	store "github.com/nexus-research-lab/nexus/internal/storage/echo"
)

const echoRequestTimeout = 30 * time.Second

type agentService interface {
	EnsureReady(context.Context) error
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type sessionService interface {
	GetSession(context.Context, string) (*protocol.Session, error)
	GetSessionMessages(context.Context, string) ([]protocol.Message, error)
	GetEchoOverride(context.Context, string) (echodomain.SessionOverride, error)
	UpdateEchoOverride(context.Context, string, echodomain.SessionOverride) (echodomain.SessionOverride, error)
}

type providerResolver interface {
	ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error)
}

type preferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
	SetEchoEnabled(context.Context, string, bool) (preferencessvc.Preferences, error)
}

type dmService interface {
	HandleChat(context.Context, dmsvc.Request) error
	HandleInterrupt(context.Context, dmsvc.InterruptRequest) error
}

// Service 负责用户级 Echo 开关和 durable due work。
type Service struct {
	config     config.Config
	repository *store.Repository
	agents     agentService
	sessions   sessionService
	dm         dmService
	runtime    *runtimectx.Manager
	providers  providerResolver
	prefs      preferencesService
	llmClient  *llm.Client
	logger     *slog.Logger
	loop       *duework.Loop
	nowFn      func() time.Time

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewService 创建 Echo 服务。
func NewService(
	cfg config.Config,
	db *sql.DB,
	agents agentService,
	sessions sessionService,
	dm dmService,
	runtime *runtimectx.Manager,
	providers providerResolver,
	prefs preferencesService,
) *Service {
	service := &Service{
		config:     cfg,
		repository: store.NewRepository(cfg, db),
		agents:     agents,
		sessions:   sessions,
		dm:         dm,
		runtime:    runtime,
		providers:  providers,
		prefs:      prefs,
		llmClient:  llm.NewClient(&http.Client{Timeout: echoRequestTimeout}),
		logger:     logx.NewDiscardLogger(),
		nowFn:      func() time.Time { return time.Now().UTC() },
	}
	service.loop = duework.New(duework.Options{
		Now: service.nowFn,
		OnError: func(err error) {
			service.logger.Warn("Echo 调度失败，等待重试", "err", err)
		},
	})
	return service
}

// SetLogger 注入业务日志。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// Start 启动 Echo deadline loop。
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()
	if s.agents != nil {
		if err := s.agents.EnsureReady(ctx); err != nil {
			s.mu.Lock()
			s.started = false
			s.mu.Unlock()
			return err
		}
	}
	if err := s.repository.RecoverInFlight(ctx); err != nil {
		s.mu.Lock()
		s.started = false
		s.mu.Unlock()
		return err
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.loop.Run(loopCtx, s.reconcile); err != nil && loopCtx.Err() == nil {
			s.logger.Error("Echo 调度器已停止", "err", err)
		}
	}()
	return nil
}

// Stop 停止 Echo deadline loop。
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.started = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// GetSettings 返回当前用户的 Echo 全局开关。
func (s *Service) GetSettings(ctx context.Context) (echodomain.Settings, error) {
	ownerUserID := authctx.OwnerUserID(ctx)
	preferences, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return echodomain.Settings{}, err
	}
	return echodomain.Settings{Enabled: preferences.EchoEnabled}, nil
}

// UpdateSettings 更新当前用户的 Echo 全局开关。
func (s *Service) UpdateSettings(ctx context.Context, input echodomain.Settings) (echodomain.Settings, error) {
	ownerUserID := authctx.OwnerUserID(ctx)
	preferences, err := s.prefs.SetEchoEnabled(ctx, ownerUserID, input.Enabled)
	if err != nil {
		return echodomain.Settings{}, err
	}
	if !preferences.EchoEnabled {
		roundIDs, cancelErr := s.repository.CancelOwner(ctx, ownerUserID)
		if cancelErr != nil {
			return echodomain.Settings{}, cancelErr
		}
		s.interruptRounds(ctx, "", roundIDs)
	}
	s.loop.Notify()
	return echodomain.Settings{Enabled: preferences.EchoEnabled}, nil
}

func (s *Service) globalPolicy(ctx context.Context, ownerUserID string) (echodomain.Policy, error) {
	policy := echodomain.DefaultPolicy(s.config.DefaultTimezone)
	preferences, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return echodomain.Policy{}, err
	}
	policy.Enabled = preferences.EchoEnabled
	return policy, nil
}

// GetSessionOverride 返回 DM 的 Echo 覆盖。
func (s *Service) GetSessionOverride(ctx context.Context, sessionKey string) (echodomain.SessionOverride, error) {
	override, err := s.sessions.GetEchoOverride(ctx, sessionKey)
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	return s.resolveSessionOverride(ctx, sessionKey, override)
}

// UpdateSessionOverride 更新 DM 的 Echo 覆盖。
func (s *Service) UpdateSessionOverride(
	ctx context.Context,
	sessionKey string,
	override echodomain.SessionOverride,
) (echodomain.SessionOverride, error) {
	updated, err := s.sessions.UpdateEchoOverride(ctx, sessionKey, override)
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	if updated.Mode == echodomain.SessionModeDisabled {
		roundIDs, cancelErr := s.OnUserActivity(ctx, authctx.OwnerUserID(ctx), sessionKey)
		if cancelErr != nil {
			return echodomain.SessionOverride{}, cancelErr
		}
		s.interruptRounds(ctx, sessionKey, roundIDs)
	}
	s.loop.Notify()
	return s.resolveSessionOverride(ctx, sessionKey, updated)
}

func (s *Service) resolveSessionOverride(
	ctx context.Context,
	sessionKey string,
	override echodomain.SessionOverride,
) (echodomain.SessionOverride, error) {
	session, err := s.sessions.GetSession(ctx, sessionKey)
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	if session == nil {
		return echodomain.SessionOverride{}, sql.ErrNoRows
	}
	if protocol.NormalizeSessionKeyChannelSegment(session.ChannelType) != protocol.SessionChannelWebSocketSegment ||
		strings.TrimSpace(session.ChatType) != protocol.RoomTypeDM {
		return echodomain.SessionOverride{}, echodomain.ErrUnsupportedSession
	}
	policy, err := s.globalPolicy(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	enabled := policy.Enabled
	if override.Mode == echodomain.SessionModeEnabled {
		enabled = true
	} else if override.Mode == echodomain.SessionModeDisabled {
		enabled = false
	}
	override.EffectiveEnabled = enabled
	return override, nil
}

// OnUserActivity 取消该 DM 尚未提交的 Echo；精确中断由 DM 主链执行。
func (s *Service) OnUserActivity(ctx context.Context, ownerUserID string, sessionKey string) ([]string, error) {
	return s.repository.CancelSession(ctx, ownerUserID, sessionKey)
}

func (s *Service) interruptRounds(ctx context.Context, sessionKey string, roundIDs []string) {
	if s.dm == nil {
		return
	}
	for _, roundID := range roundIDs {
		roundSessionKey := strings.TrimSpace(sessionKey)
		if roundSessionKey == "" {
			attempt, err := s.repository.GetAttemptByRuntimeRoundID(ctx, roundID)
			if err == nil && attempt != nil {
				roundSessionKey = attempt.SessionKey
			}
		}
		if roundSessionKey == "" {
			continue
		}
		if err := s.dm.HandleInterrupt(ctx, dmsvc.InterruptRequest{SessionKey: roundSessionKey, RoundID: roundID}); err != nil {
			s.logger.Debug("Echo round 已结束或无法中断", "session_key", roundSessionKey, "round_id", roundID, "err", err)
		}
	}
}

func ownerContext(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" || authctx.OwnerUserID(ctx) == ownerUserID {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "echo",
	})
}
