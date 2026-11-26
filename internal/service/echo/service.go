// INPUT: 用户 Preferences、Agent/Session 读取、Provider 解析与轻量模型。
// OUTPUT: 带 CAS revision 的用户级开关，以及主窗口要的两个判断：何时安排 Echo、到点时是否开口。
// POS: Echo 应用服务装配根；状态全在主窗口 inbox 的 echo 事件上，本服务不持有调度与尝试记录。
package echo

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

const echoRequestTimeout = 30 * time.Second

type agentService interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type sessionService interface {
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

// Service 提供 Echo 开关与判断；实现 dm.EchoPolicy。
type Service struct {
	config    config.Config
	agents    agentService
	sessions  sessionService
	providers providerResolver
	prefs     preferencesService
	llmClient *llm.Client
	logger    *slog.Logger
	nowFn     func() time.Time
}

var _ dmsvc.EchoPolicy = (*Service)(nil)

// NewService 创建 Echo 服务。
func NewService(
	cfg config.Config,
	agents agentService,
	sessions sessionService,
	providers providerResolver,
	prefs preferencesService,
) *Service {
	return &Service{
		config:    cfg,
		agents:    agents,
		sessions:  sessions,
		providers: providers,
		prefs:     prefs,
		llmClient: llm.NewClient(&http.Client{Timeout: echoRequestTimeout}),
		logger:    logx.NewDiscardLogger(),
		nowFn:     func() time.Time { return time.Now().UTC() },
	}
}

// SetLogger 注入业务日志。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// GetSettings 返回当前用户的 Echo 全局开关。
func (s *Service) GetSettings(ctx context.Context) (echodomain.Settings, error) {
	preferences, err := s.prefs.Get(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return echodomain.Settings{}, err
	}
	return echoSettings(preferences), nil
}

// UpdateSettings 更新当前用户的 Echo 全局开关；关闭后已安排的 Echo 到点即静默，无需另行收口。
func (s *Service) UpdateSettings(ctx context.Context, input echodomain.Settings) (echodomain.Settings, error) {
	preferences, err := s.prefs.SetEchoEnabled(ctx, authctx.OwnerUserID(ctx), input.Enabled)
	if err != nil {
		return echodomain.Settings{}, err
	}
	return echoSettings(preferences), nil
}

// UpdateSettingsAtVersion 以 Preferences revision 为条件更新 Echo 开关。
func (s *Service) UpdateSettingsAtVersion(
	ctx context.Context,
	input echodomain.Settings,
	expectedVersion int64,
) (echodomain.Settings, error) {
	preferences, err := s.prefs.SetEchoEnabledAtVersion(ctx, authctx.OwnerUserID(ctx), input.Enabled, expectedVersion)
	if err != nil {
		return echodomain.Settings{}, err
	}
	return echoSettings(preferences), nil
}

// ScheduleAfter 返回一轮对话结束后的 Echo 到点时间：安静等待后，顺延到活跃时段内。
func (s *Service) ScheduleAfter(ctx context.Context, ownerUserID string, finishedAt time.Time) (time.Time, bool, error) {
	policy, err := s.policy(ctx, ownerUserID)
	if err != nil || !policy.Enabled {
		return time.Time{}, false, err
	}
	dueAt, err := nextActiveTime(finishedAt.Add(time.Duration(policy.IdleDelaySeconds)*time.Second), policy)
	if err != nil {
		return time.Time{}, false, err
	}
	return dueAt, true, nil
}

// Decide 在到点时判断是否开口；任何不确定都返回沉默，基础设施错误同时上报。
func (s *Service) Decide(ctx context.Context, candidate dmsvc.EchoCandidate) (dmsvc.EchoDecision, error) {
	ctx = ownerContext(ctx, candidate.OwnerUserID)
	policy, err := s.policy(ctx, candidate.OwnerUserID)
	if err != nil {
		return dmsvc.EchoDecision{Reason: "internal_error"}, err
	}
	if !policy.Enabled {
		return dmsvc.EchoDecision{Reason: "disabled"}, nil
	}
	active, nextActiveAt, err := activeWindow(s.nowFn(), policy)
	if err != nil {
		return dmsvc.EchoDecision{Reason: "internal_error"}, err
	}
	if !active {
		return dmsvc.EchoDecision{DeferUntil: nextActiveAt, Reason: "outside_active_hours"}, nil
	}
	history, err := s.sessions.GetSessionMessages(ctx, candidate.SessionKey)
	if err != nil {
		return dmsvc.EchoDecision{Reason: "internal_error"}, err
	}
	decision, err := s.evaluateConversation(ctx, candidate, history)
	if err != nil {
		return dmsvc.EchoDecision{Reason: "gate_unavailable"}, err
	}
	if decision.Decision != gateFollowUp {
		return dmsvc.EchoDecision{Reason: decision.ReasonCode}, nil
	}
	return dmsvc.EchoDecision{Prompt: buildFollowUpPrompt(decision.Focus), Reason: decision.ReasonCode}, nil
}

func (s *Service) policy(ctx context.Context, ownerUserID string) (echodomain.Policy, error) {
	policy := echodomain.DefaultPolicy(s.config.DefaultTimezone)
	preferences, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return echodomain.Policy{}, err
	}
	policy.Enabled = preferences.EchoEnabled
	return policy, nil
}

func echoSettings(preferences preferencessvc.Preferences) echodomain.Settings {
	return echodomain.Settings{Enabled: preferences.EchoEnabled, Version: preferences.Version}
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
