// INPUT: automation 配置、持久化连接、Agent/Room/统一 Session/投递依赖与 artifact 删除协调器。
// OUTPUT: 调度、任务控制、heartbeat、运行态编排与 isolated Session tombstone 清理服务。
// POS: automation 服务的依赖装配与进程内状态根。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/config"
	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
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
	HandleChat(context.Context, roomrealtime.ChatRequest) error
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
}

type roomInterruptRunner interface {
	HandleInterrupt(context.Context, roomrealtime.InterruptRequest) error
}

type workspaceReader interface {
	GetFile(context.Context, string, string) (*workspacepkg.FileContent, error)
}

type deliveryRouter interface {
	DeliverMessage(context.Context, string, string, channels.DeliveryTarget) (channels.DeliveryResult, error)
}

type automationResultDeliveryRouter interface {
	DeliverAutomationResult(
		context.Context,
		string,
		string,
		channels.DeliveryTarget,
		channels.AutomationDeliveryContext,
	) (channels.DeliveryResult, error)
}

type deliveryGrantResolver interface {
	ValidateAutomationDeliveryGrant(context.Context, string, string, string) error
}

type deliverySessionResolver interface {
	ResolveDeliverySession(context.Context, string) (*protocol.Session, error)
}

type imagegenDefaultResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

type agentAuthority interface {
	EnsureReady(context.Context) error
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type connectorConnectionResolver interface {
	LoadActiveConnection(context.Context, string, string) (*connectordomain.ConnectionSnapshot, error)
}

// ErrSessionArtifactDeletionCoordinatorUnavailable 表示 isolated Session 清理缺少统一协调器。
var ErrSessionArtifactDeletionCoordinatorUnavailable = errors.New(
	"Automation Session artifact 删除协调器未装配",
)

// SessionArtifactDeletionCoordinator 统一撤销 isolated Automation Session 的 runtime 与持久 artifact。
type SessionArtifactDeletionCoordinator interface {
	DeleteSessionArtifacts(context.Context, string, string, string, string) error
}

// TaskEventNotifier 接收定时任务变更事件。
type TaskEventNotifier interface {
	NotifyTaskEvent(context.Context, automationdomain.ScheduledTaskEvent)
}

// TaskEventNotifierFunc 适配函数式定时任务事件通知器。
type TaskEventNotifierFunc func(context.Context, automationdomain.ScheduledTaskEvent)

// NotifyTaskEvent 实现 TaskEventNotifier。
func (fn TaskEventNotifierFunc) NotifyTaskEvent(ctx context.Context, event automationdomain.ScheduledTaskEvent) {
	if fn != nil {
		fn(ctx, event)
	}
}

// Service 提供 scheduled tasks 与 heartbeat 的真实业务能力。
type Service struct {
	config           config.Config
	repository       *automationstore.Repository
	agents           agentAuthority
	dm               dmRunner
	room             roomRunner
	permission       *permissionctx.Context
	providers        imagegenDefaultResolver
	connectors       connectorConnectionResolver
	workspace        workspaceReader
	delivery         deliveryRouter
	deliveryGrants   deliveryGrantResolver
	deliverySessions deliverySessionResolver
	logger           *slog.Logger
	sessionArtifacts SessionArtifactDeletionCoordinator
	taskNotifier     TaskEventNotifier

	nowFn     func() time.Time
	idFactory func(string) string

	// taskControlMu serializes human-control-plane changes with Agent-initiated
	// task mutations/runs so the script capability check and the real action
	// observe one process-local task-control order.
	taskControlMu         sync.Mutex
	heartbeatControlMu    sync.Mutex
	mu                    sync.Mutex
	jobStates             map[string]*automationexec.JobRuntimeState
	heartbeatState        map[string]*automationexec.HeartbeatRuntimeState
	wakeRequests          map[string][]automationexec.HeartbeatWakeRequest
	attemptMu             sync.Mutex
	physicalAttempts      map[physicalAttemptKey]*physicalAttempt
	deliveryRetryRunning  bool
	schedulerOwnerID      string
	schedulerLeaseHeld    bool
	schedulerLeaseRenewAt time.Time
	started               bool
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
}

// SetDeliveryGrantResolver 注入 IM pairing 的实时授权检查器。
func (s *Service) SetDeliveryGrantResolver(resolver deliveryGrantResolver) {
	s.deliveryGrants = resolver
}

// SetDeliverySessionResolver 注入 Nexus 统一 Session 读模型，使数据库拥有的
// Room-backed DM/成员会话与 workspace/IM Session 共用同一存在性边界。
func (s *Service) SetDeliverySessionResolver(resolver deliverySessionResolver) {
	s.deliverySessions = resolver
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
	var authority agentAuthority
	if agents != nil {
		authority = agents
	}
	return &Service{
		config:           cfg,
		repository:       automationstore.NewRepository(cfg, db),
		agents:           authority,
		dm:               dm,
		room:             room,
		permission:       permission,
		workspace:        workspace,
		delivery:         delivery,
		logger:           logx.NewDiscardLogger(),
		nowFn:            func() time.Time { return time.Now().UTC() },
		idFactory:        automationexec.NewID,
		schedulerOwnerID: automationexec.NewID("scheduler"),
		jobStates:        make(map[string]*automationexec.JobRuntimeState),
		heartbeatState:   make(map[string]*automationexec.HeartbeatRuntimeState),
		wakeRequests:     make(map[string][]automationexec.HeartbeatWakeRequest),
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

// SetSessionArtifactDeletionCoordinator 注入 isolated Session 的统一删除协调器。
func (s *Service) SetSessionArtifactDeletionCoordinator(
	coordinator SessionArtifactDeletionCoordinator,
) {
	s.sessionArtifacts = coordinator
}

// SetProviderResolver 注入 Provider 解析器，用于判断后台运行时是否可默认开放图片生成工具。
func (s *Service) SetProviderResolver(resolver imagegenDefaultResolver) {
	s.providers = resolver
}

// SetConnectorResolver 注入 connector 连接检查器；授权和 OAuth readiness 保持独立判定。
func (s *Service) SetConnectorResolver(resolver connectorConnectionResolver) {
	s.connectors = resolver
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
			s.mu.Lock()
			s.started = false
			s.mu.Unlock()
			return err
		}
	}
	if err := s.bootstrapRuntime(ctx); err != nil {
		s.mu.Lock()
		s.started = false
		s.mu.Unlock()
		return err
	}
	if _, _, err := s.refreshSchedulerLease(ctx, s.nowFn()); err != nil {
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
	if err := s.releaseSchedulerLease(context.Background()); err != nil {
		s.loggerFor(context.Background()).Warn("释放自动化调度器租约失败", "err", err)
	}
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

func (s *Service) validateAgentAndTarget(ctx context.Context, agentID string, target automationdomain.SessionTarget) error {
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return err
	}
	if strings.TrimSpace(target.Kind) != automationdomain.SessionTargetBound {
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

func (s *Service) ensureDirectTargetSupported(target automationdomain.SessionTarget) error {
	if strings.TrimSpace(target.Kind) == automationdomain.SessionTargetMain {
		return nil
	}
	_, err := automationexec.ResolveSessionKey(automationdomain.ScheduledTask{
		AgentID:       "noop",
		SessionTarget: target,
	}, stringPointer("noop"))
	return err
}
