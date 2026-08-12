// INPUT: Room 服务依赖、runtime 事件与实时会话请求。
// OUTPUT: Room round、队列和共享事件的进程内编排状态。
// POS: Room 实时服务装配与共享状态定义。
package realtime

import (
	"context"
	"errors"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
)

const (
	interruptForceCancelDelay = 150 * time.Millisecond
	roomBroadcastTimeout      = 5 * time.Second
)

// ErrRoomRuntimeRequiresGroup 表示 DM 被错误路由到了 Room 执行域。
var ErrRoomRuntimeRequiresGroup = errors.New("room realtime execution requires group room")

func requireGroupRoomContext(contextValue *protocol.ConversationContextAggregate) error {
	if contextValue == nil {
		return errors.New("room conversation not found")
	}
	if contextValue.Room.RoomType != protocol.RoomTypeGroup {
		return ErrRoomRuntimeRequiresGroup
	}
	return nil
}

type roomClientFactory interface {
	New(agentclient.Options) runtimectx.Client
}

// RoomBroadcaster 负责把 Room 共享事件扇出到房间级订阅者。
type RoomBroadcaster interface {
	Broadcast(context.Context, string, protocol.EventMessage) []error
}

// RoomEventObserver 接收 Room 共享事件的内部镜像，用于后台自动化等非 UI 消费者。
type RoomEventObserver func(context.Context, protocol.EventMessage)

type defaultRoomClientFactory struct{}

func (f defaultRoomClientFactory) New(options agentclient.Options) runtimectx.Client {
	return runtimectx.NewAgentClient(options)
}

// ChatRequest 表示 Room 共享会话的一次聊天请求。
// RoundID / UserMessageID 由后端 mint：WS 入口不填，HandleChat 内部生成；
// 后端内部调用方（automation / mention / queue）可预置 RoundID。
type ChatRequest struct {
	SessionKey            string
	RoomID                string
	ConversationID        string
	CoordinatorAgentID    string
	AttachmentAgentID     string
	Content               string
	GoalContext           string
	GoalID                string
	GoalObjectiveRevision int64
	ExecutionID           string
	Attachments           []protocol.ChatAttachment
	TargetAgentIDs        []string
	ClientRequestID       string
	ClientMessageID       string
	RoundID               string
	UserMessageID         string
	DeliveryPolicy        protocol.ChatDeliveryPolicy
	BroadcastUserMessage  bool
	Internal              bool
	// TrustedConfigurationContext 仅由 Nexus WebSocket 用户入口设置，后台/Agent wake/队列不得继承。
	TrustedConfigurationContext bool
	// ExecutionOrigin 由服务端调度器写入；非空值不会获得持久配置 capability。
	ExecutionOrigin string
	// trustedQueuedConfigurationContext 只能由本包在成功 claim 宿主 DB
	// admission 后设置，外部 ChatRequest 构造者无法伪造。
	trustedQueuedConfigurationContext bool
	InputOptions                      sdkprotocol.OutboundMessageOptions
	PermissionMode                    sdkpermission.Mode
	PermissionHandler                 sdkpermission.Handler
	EventObserver                     RoomEventObserver
}

// InterruptRequest 表示 Room 会话中断请求。按 root round + agent slot 定位执行对象。
type InterruptRequest struct {
	SessionKey   string
	RoundID      string
	AgentRoundID string
}

// MCPServerBuilder 由 server app 注入，按当前会话上下文构造一组 MCP server。
// 用 string 形参避免 room domain 反向依赖 automation 子包，防止 import cycle。
type MCPServerBuilder func(
	ctx context.Context,
	agentValue *protocol.Agent,
	sessionKey string,
	roundID string,
	sourceContextType string,
	sourceContextID string,
	sourceContextLabel string,
	goalObjectiveRevision *atomic.Int64,
	permissionMode sdkpermission.Mode,
) map[string]sdkmcp.ServerConfig

// roomContextStore 是 realtime 读取和更新持久化 Room 状态所需的最小能力集。
type roomContextStore interface {
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
	GetConversationContextForSystem(context.Context, string) (*protocol.ConversationContextAggregate, error)
	UpdateSessionSDKSessionID(context.Context, string, string) error
	TouchConversationActivity(context.Context, string, time.Time) error
	MarkConversationStarted(context.Context, string, time.Time) error
	BuildRoomSkillPrompt(context.Context, []string) (string, error)
}

type Service struct {
	config              config.Config
	rooms               roomContextStore
	agents              *agentsvc.Service
	runtime             *runtimectx.Manager
	permission          *permissionctx.Context
	providers           clientopts.RuntimeConfigResolver
	admission           clientopts.AgentRuntimeAdmissionResolver
	prefs               roomRuntimePreferencesService
	files               *workspacestore.SessionFileStore
	history             *workspacestore.AgentHistoryStore
	roomHistory         *workspacestore.RoomHistoryStore
	directedMessages    *workspacestore.RoomDirectedMessageStore
	directedWakes       *workspacestore.RoomDirectedMessageWakeStore
	publicHandoffs      *workspacestore.RoomPublicHandoffStore
	inputQueue          *workspacestore.InputQueueStore
	queueTrust          queueAdmissionStore
	usage               usageRecorder
	quota               quotaChecker
	goals               goalContextProvider
	executionContext    executionContextProvider
	subagentAdmission   orchestrationruntimehook.Provider
	factory             roomClientFactory
	broadcaster         RoomBroadcaster
	logger              *slog.Logger
	mcpServers          MCPServerBuilder
	executionMCPServers runtimectx.ExecutionMCPServerBuilder
	titles              roomTitleScheduler

	// goalUsageRetryBaseDelay 为零时使用生产退避；测试只调整时钟尺度。
	goalUsageRetryBaseDelay time.Duration

	rounds              roomRoundRegistry
	goalUsageScopeLocks roomGoalUsageScopeLockRegistry
	wakeTimers          *roomWakeTimerRegistry
}

type roomTitleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

type roomRuntimePreferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

type queueAdmissionStore interface {
	Record(context.Context, queueadmissionstore.Admission) error
	Claim(context.Context, queueadmissionstore.Binding) (queueadmissionstore.Claim, bool, error)
	Release(context.Context, queueadmissionstore.Claim) error
	Consume(context.Context, queueadmissionstore.Claim) error
	Revoke(context.Context, queueadmissionstore.Binding) error
}

type usageRecorder interface {
	RecordMessageUsage(context.Context, usagesvc.RecordInput) error
}

type quotaChecker interface {
	EnsureQuotaAvailable(context.Context, string) error
}

type goalContextProvider interface {
	RuntimeContext(context.Context, string) (string, *protocol.Goal, error)
	RecordUsageForSession(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	RecordUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error)
	RecordContinuationProgress(context.Context, string, string, bool, ...int64) (*protocol.Goal, error)
	RecordContinuationFailure(context.Context, string, string, string, ...int64) (*protocol.Goal, error)
	RecordCompletionToolMiss(context.Context, string, string, string, ...int64) (*protocol.Goal, error)
	RecordGoalActivity(context.Context, string, string, ...int64) (*protocol.Goal, error)
	RecordRoomGoalCollaborationHandback(context.Context, string, string, ...int64) (*protocol.Goal, error)
	RecordRoomGoalCollaborationRequired(context.Context, string, string) (*protocol.Goal, error)
	RecordRoomGoalCollaborationEvidence(context.Context, string, string, string, ...int64) (*protocol.Goal, error)
}

type goalEventProvider interface {
	Events(context.Context, string, int) ([]protocol.GoalEvent, error)
}

type goalContinuationProvider interface {
	PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error)
	GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error)
	ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error)
}

// NewService 创建 Room 实时编排服务。
func NewService(
	cfg config.Config,
	roomService roomContextStore,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
) *Service {
	return NewServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, defaultRoomClientFactory{})
}

// NewServiceWithFactory 使用自定义客户端工厂创建服务。
func NewServiceWithFactory(
	cfg config.Config,
	roomService roomContextStore,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
	factory roomClientFactory,
) *Service {
	if factory == nil {
		factory = defaultRoomClientFactory{}
	}
	return &Service{
		config:              cfg,
		rooms:               roomService,
		agents:              agentService,
		runtime:             runtimeManager,
		permission:          permission,
		files:               workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:             workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		roomHistory:         workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
		directedMessages:    workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath),
		directedWakes:       workspacestore.NewRoomDirectedMessageWakeStore(cfg.WorkspacePath),
		publicHandoffs:      workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath),
		inputQueue:          workspacestore.NewInputQueueStore(cfg.WorkspacePath),
		factory:             factory,
		logger:              logx.NewDiscardLogger(),
		rounds:              newRoomRoundRegistry(),
		goalUsageScopeLocks: newRoomGoalUsageScopeLockRegistry(),
		wakeTimers:          newRoomWakeTimerRegistry(),
	}
}

// SetRoomBroadcaster 注入 Room 共享事件广播器。
func (s *Service) SetRoomBroadcaster(broadcaster RoomBroadcaster) {
	s.broadcaster = broadcaster
	if s.permission != nil {
		s.permission.SetRoomBroadcaster(broadcaster)
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

// SetProviderResolver 注入 Provider 运行时解析器。
func (s *Service) SetProviderResolver(resolver clientopts.RuntimeConfigResolver) {
	s.providers = resolver
}

// SetRuntimeAdmissionResolver 注入认证转场与动态强隔离 admission。
func (s *Service) SetRuntimeAdmissionResolver(
	resolver clientopts.AgentRuntimeAdmissionResolver,
) {
	s.admission = resolver
}

// SetPreferences 注入用户偏好服务，用于 Agent 未显式选模型时读取默认对话模型。
func (s *Service) SetPreferences(prefs roomRuntimePreferencesService) {
	s.prefs = prefs
}

// SetQueueAdmissionStore 注入宿主 DB 中不可由 Agent workspace 伪造的队列信任根。
func (s *Service) SetQueueAdmissionStore(store queueAdmissionStore) {
	s.queueTrust = store
}

// SetUsageRecorder 注入 token usage 持久化 ledger。
func (s *Service) SetUsageRecorder(recorder usageRecorder) {
	s.usage = recorder
}

// SetQuotaChecker 注入订阅额度检查器。
func (s *Service) SetQuotaChecker(checker quotaChecker) {
	s.quota = checker
}

func (s *Service) ensureQuotaAvailable(ctx context.Context) error {
	if s.quota == nil {
		return nil
	}
	return s.quota.EnsureQuotaAvailable(ctx, authctx.OwnerUserID(ctx))
}

// SetGoalContextProvider 注入 Goal runtime context provider。
func (s *Service) SetGoalContextProvider(provider goalContextProvider) {
	s.goals = provider
}

// SetMCPServerBuilder 注入按会话上下文构造 MCP server 的工厂。
func (s *Service) SetMCPServerBuilder(builder MCPServerBuilder) {
	s.mcpServers = builder
}

// SetExecutionMCPServerBuilder 注入需要完整 slot/round identity 的 Execution MCP overlay。
func (s *Service) SetExecutionMCPServerBuilder(builder runtimectx.ExecutionMCPServerBuilder) {
	s.executionMCPServers = builder
}

// SetSubagentAdmissionProvider 注入 Agent tool 的权威 WorkGraph 准入与 Attempt lifecycle。
func (s *Service) SetSubagentAdmissionProvider(provider orchestrationruntimehook.Provider) {
	s.subagentAdmission = provider
}

// SetTitleGenerator 注入会话标题生成器。
func (s *Service) SetTitleGenerator(generator roomTitleScheduler) {
	s.titles = generator
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

// INPUT: Room conversation ID 与调用方身份。
// OUTPUT: 用户可见或系统恢复使用的 Room conversation 聚合。
// POS: 实时编排读取持久化 Room 上下文的唯一适配边界。
// GetConversationContext 暴露 Room conversation 聚合，供 automation 做目标成员校验。
func (s *Service) GetConversationContext(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	if s.rooms == nil {
		return nil, errors.New("room service is not configured")
	}
	return s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
}

func (s *Service) internalConversationContext(
	ctx context.Context,
	conversationID string,
	internal bool,
) (context.Context, *protocol.ConversationContextAggregate, error) {
	if s.rooms == nil {
		return ctx, nil, errors.New("room service is not configured")
	}
	if !internal {
		contextValue, err := s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
		return ctx, contextValue, err
	}
	if _, ok := authctx.CurrentUserID(ctx); ok {
		contextValue, err := s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
		return ctx, contextValue, err
	}
	contextValue, err := s.rooms.GetConversationContextForSystem(ctx, strings.TrimSpace(conversationID))
	if err != nil || contextValue == nil {
		return ctx, contextValue, err
	}
	ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
	if ownerUserID == "" {
		return ctx, contextValue, nil
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	}), contextValue, nil
}

func (s *Service) withBroadcastTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, roomBroadcastTimeout)
}

func (s *Service) broadcastSharedEventWithTimeout(
	ctx context.Context,
	sessionKey string,
	roomID string,
	event protocol.EventMessage,
) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	s.broadcastSharedEvent(broadcastCtx, sessionKey, roomID, event)
}

func (s *Service) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	if errs := s.permission.BroadcastSessionStatus(
		broadcastCtx,
		sessionKey,
		s.runtime.GetRunningRoundIDs(sessionKey),
	); len(errs) > 0 {
		s.loggerFor(broadcastCtx).Warn("广播 Room session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *Service) broadcastSharedEvent(ctx context.Context, sessionKey string, roomID string, event protocol.EventMessage) {
	if s.broadcaster != nil && strings.TrimSpace(roomID) != "" {
		s.broadcaster.Broadcast(ctx, roomID, event)
		// RoomBroadcaster 面向房间 WebSocket，不经过 permission.Context；后台自动化需要这条内部镜像。
		s.notifyRoomEventObserver(ctx, sessionKey, event)
		return
	}
	s.permission.BroadcastEvent(ctx, sessionKey, event)
}

func (s *Service) notifyRoomEventObserver(ctx context.Context, sessionKey string, event protocol.EventMessage) {
	if s == nil {
		return
	}
	roundID := eventRoundID(event)
	if strings.TrimSpace(roundID) == "" {
		return
	}
	roundValue := s.rounds.findByRoundID(sessionKey, roundID)
	var observer RoomEventObserver
	if roundValue != nil {
		observer = roundValue.EventObserver
	}
	if observer == nil {
		return
	}
	observer(ctx, event)
}

func eventRoundID(event protocol.EventMessage) string {
	if roundID := strings.TrimSpace(anyString(event.Data["round_id"])); roundID != "" {
		return roundID
	}
	return strings.TrimSpace(event.RoundID)
}
