// INPUT: Echo attempt 仓储、用户 Preferences、Agent/Session/DM 服务、轻量模型与 runtime 状态。
// OUTPUT: 带 CAS revision 的用户级开关、提交后收口证据、用户活动取消与后台调度生命周期。
// POS: Echo 应用服务装配根。
package echo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

const (
	echoRequestTimeout          = 30 * time.Second
	echoSettingsFinalizeTimeout = 15 * time.Second
)

type agentService interface {
	EnsureReady(context.Context) error
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type sessionService interface {
	GetSession(context.Context, string) (*protocol.Session, error)
	GetSessionMessages(context.Context, string) ([]protocol.Message, error)
}

type providerResolver interface {
	ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error)
}

type preferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
	SetEchoEnabled(context.Context, string, bool) (preferencessvc.Preferences, error)
	SetEchoEnabledAtVersion(context.Context, string, bool, int64) (preferencessvc.Preferences, error)
}

// ErrSettingsVersionConflict 表示 Echo 所在的 Preferences aggregate 已被其他设置更新。
var ErrSettingsVersionConflict = preferencessvc.ErrVersionConflict

// SettingsReconcileError 表示开关已经保存，但停用后的在途尝试收口尚未完成。
type SettingsReconcileError struct {
	Cause error
}

func (e *SettingsReconcileError) Error() string {
	return fmt.Sprintf("Echo 设置已保存，但在途尝试收口失败: %v", e.Cause)
}

func (e *SettingsReconcileError) Unwrap() error { return e.Cause }

// SettingsUpdateCommitted 只根据服务阶段证据判断开关是否已经保存。
func SettingsUpdateCommitted(err error) bool {
	var reconcileErr *SettingsReconcileError
	return errors.As(err, &reconcileErr)
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
	return echoSettings(preferences), nil
}

// UpdateSettings 更新当前用户的 Echo 全局开关。
func (s *Service) UpdateSettings(ctx context.Context, input echodomain.Settings) (echodomain.Settings, error) {
	return s.updateSettings(ctx, input, nil)
}

// UpdateSettingsAtVersion 以 Preferences revision 为条件更新 Echo 开关。
func (s *Service) UpdateSettingsAtVersion(
	ctx context.Context,
	input echodomain.Settings,
	expectedVersion int64,
) (echodomain.Settings, error) {
	return s.updateSettings(ctx, input, &expectedVersion)
}

func (s *Service) updateSettings(
	ctx context.Context,
	input echodomain.Settings,
	expectedVersion *int64,
) (echodomain.Settings, error) {
	ownerUserID := authctx.OwnerUserID(ctx)
	var preferences preferencessvc.Preferences
	var err error
	if expectedVersion == nil {
		preferences, err = s.prefs.SetEchoEnabled(ctx, ownerUserID, input.Enabled)
	} else {
		preferences, err = s.prefs.SetEchoEnabledAtVersion(
			ctx,
			ownerUserID,
			input.Enabled,
			*expectedVersion,
		)
	}
	if err != nil {
		return echodomain.Settings{}, err
	}
	settings := echoSettings(preferences)
	defer s.loop.Notify()
	if !preferences.EchoEnabled {
		// The setting is already durable. Navigation or a dropped response must not
		// cancel the safety step that fences scheduled/running follow-ups.
		finalizeCtx, cancel := newSettingsFinalizeContext(ctx)
		defer cancel()
		roundIDs, cancelErr := s.repository.CancelOwner(finalizeCtx, ownerUserID)
		if cancelErr != nil {
			return settings, &SettingsReconcileError{Cause: cancelErr}
		}
		s.interruptRounds(finalizeCtx, "", roundIDs)
	}
	return settings, nil
}

func newSettingsFinalizeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), echoSettingsFinalizeTimeout)
}

func echoSettings(preferences preferencessvc.Preferences) echodomain.Settings {
	return echodomain.Settings{
		Enabled: preferences.EchoEnabled,
		Version: preferences.Version,
	}
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
