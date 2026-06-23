package automation

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

type dmRunner interface {
	HandleChat(context.Context, dmsvc.Request) error
}

type dmInterruptRunner interface {
	HandleInterrupt(context.Context, dmsvc.InterruptRequest) error
}

type roomRunner interface {
	HandleChat(context.Context, roomsvc.ChatRequest) error
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
}

type roomInterruptRunner interface {
	HandleInterrupt(context.Context, roomsvc.InterruptRequest) error
}

type workspaceReader interface {
	GetFile(context.Context, string, string) (*workspacepkg.FileContent, error)
}

type deliveryRouter interface {
	DeliverMessage(context.Context, string, string, channels.DeliveryTarget) (channels.DeliveryResult, error)
}

type runtimeSessionCloser interface {
	CloseSession(context.Context, string) error
}

type imagegenDefaultResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

// TaskEventNotifier 接收定时任务变更事件。
type TaskEventNotifier interface {
	NotifyTaskEvent(context.Context, protocol.CronTaskEvent)
}

// TaskEventNotifierFunc 适配函数式定时任务事件通知器。
type TaskEventNotifierFunc func(context.Context, protocol.CronTaskEvent)

// NotifyTaskEvent 实现 TaskEventNotifier。
func (fn TaskEventNotifierFunc) NotifyTaskEvent(ctx context.Context, event protocol.CronTaskEvent) {
	if fn != nil {
		fn(ctx, event)
	}
}

// Service 提供 scheduled tasks 与 heartbeat 的真实业务能力。
type Service struct {
	config        config.Config
	repository    *automationstore.Repository
	agents        *agentsvc.Service
	dm            dmRunner
	room          roomRunner
	permission    *permissionctx.Context
	providers     imagegenDefaultResolver
	workspace     workspaceReader
	delivery      deliveryRouter
	logger        *slog.Logger
	sessionCloser runtimeSessionCloser
	taskNotifier  TaskEventNotifier

	nowFn     func() time.Time
	idFactory func(string) string

	mu                   sync.Mutex
	jobStates            map[string]*automationdomain.JobRuntimeState
	heartbeatState       map[string]*automationdomain.HeartbeatRuntimeState
	wakeRequests         map[string][]automationdomain.HeartbeatWakeRequest
	deliveryRetryRunning bool
	started              bool
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
}

// NewService 创建自动化服务。
func NewService(
	cfg config.Config,
	db *sql.DB,
	agents *agentsvc.Service,
	dm dmRunner,
	room roomRunner,
	permission *permissionctx.Context,
	workspace workspaceReader,
	delivery deliveryRouter,
) *Service {
	return &Service{
		config:         cfg,
		repository:     automationstore.NewRepository(cfg, db),
		agents:         agents,
		dm:             dm,
		room:           room,
		permission:     permission,
		workspace:      workspace,
		delivery:       delivery,
		logger:         logx.NewDiscardLogger(),
		nowFn:          func() time.Time { return time.Now().UTC() },
		idFactory:      automationdomain.NewID,
		jobStates:      make(map[string]*automationdomain.JobRuntimeState),
		heartbeatState: make(map[string]*automationdomain.HeartbeatRuntimeState),
		wakeRequests:   make(map[string][]automationdomain.HeartbeatWakeRequest),
	}
}

// SetLogger 注入业务日志实例。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetRuntimeSessionCloser 注入运行时会话关闭器，用于清理 isolated 自动化会话。
func (s *Service) SetRuntimeSessionCloser(sessionCloser runtimeSessionCloser) {
	s.sessionCloser = sessionCloser
}

// SetProviderResolver 注入 Provider 解析器，用于判断后台运行时是否可默认开放图片生成工具。
func (s *Service) SetProviderResolver(resolver imagegenDefaultResolver) {
	s.providers = resolver
}

func (s *Service) runtimeImagegenDefaultEnabled(ctx context.Context) bool {
	if s == nil || s.providers == nil {
		return false
	}
	_, err := s.providers.ResolveImageConfig(ctx, "")
	return err == nil
}

// SetTaskEventNotifier 注入定时任务事件通知器。
func (s *Service) SetTaskEventNotifier(notifier TaskEventNotifier) {
	s.taskNotifier = notifier
}

// Start 启动后台调度循环。
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
			return err
		}
	}
	if err := s.bootstrapRuntime(ctx); err != nil {
		return err
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	s.loggerFor(ctx).Info("自动化调度器已启动")
	go s.runLoop(loopCtx)
	return nil
}

// Stop 停止后台调度循环。
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
	s.loggerFor(context.Background()).Info("自动化调度器已停止")
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

func (s *Service) ensureReady(ctx context.Context) error {
	if s.agents == nil {
		return nil
	}
	return s.agents.EnsureReady(ctx)
}

func (s *Service) requireAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if s.agents == nil {
		return nil, nil
	}
	return s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
}

func (s *Service) validateAgentAndTarget(ctx context.Context, agentID string, target protocol.SessionTarget) error {
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return err
	}
	if strings.TrimSpace(target.Kind) != protocol.SessionTargetBound {
		return nil
	}
	parsed := protocol.ParseSessionKey(target.BoundSessionKey)
	if parsed.Kind == protocol.SessionKeyKindAgent && parsed.AgentID != "" && parsed.AgentID != strings.TrimSpace(agentID) {
		return errors.New("agent_id 与 session_target 不一致")
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return s.validateRoomTargetAgent(ctx, parsed.ConversationID, agentID)
	}
	return nil
}

func (s *Service) validateRoomTargetAgent(ctx context.Context, conversationID string, agentID string) error {
	if s.room == nil {
		return errors.New("automation room runner is not configured")
	}
	contextValue, err := s.room.GetConversationContext(ctx, strings.TrimSpace(conversationID))
	if err != nil {
		return err
	}
	if contextValue == nil || !roomdomain.IsMemberAgent(contextValue.Members, agentID) {
		return errors.New("agent_id 不是目标 Room 的成员")
	}
	return nil
}

func (s *Service) ensureDirectTargetSupported(target protocol.SessionTarget) error {
	if strings.TrimSpace(target.Kind) == protocol.SessionTargetMain {
		return nil
	}
	_, err := automationdomain.ResolveSessionKey(protocol.CronJob{
		AgentID:       "noop",
		SessionTarget: target,
	}, stringPointer("noop"))
	return err
}
