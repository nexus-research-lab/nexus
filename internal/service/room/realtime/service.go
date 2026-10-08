// INPUT: Room 服务依赖、runtime 事件、owner-scoped Slash expander 与实时会话请求。
// OUTPUT: Room round、动态 Workflow、队列和共享事件的进程内编排状态。
// POS: Room 实时服务装配与共享状态定义。
package realtime

import (
	"context"
	"errors"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
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
	SessionKey         string
	RoomID             string
	ConversationID     string
	CoordinatorAgentID string
	AttachmentAgentID  string
	Content            string
	// PublicContext 仅由服务端在线适配提供，沿用 Room 公区游标与上下文预算。
	PublicContext []protocol.Message
	// PublicAgentDirectory 是服务端在线成员展示目录，不参与本机执行资格。
	PublicAgentDirectory  map[string]string
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
	// RuntimeToolPolicy 仅供 automation 等受控执行传入创建时权限快照。
	RuntimeToolPolicy *protocol.RuntimeToolPolicy
	// AutomationRun 只由 Automation 调度器签发，作为 runtime/MCP 的可信 run 身份。
	AutomationRun *protocol.AutomationRunContext
	EventObserver RoomEventObserver
	// continuationStartAdmission is host-only. It advances the durable
	// continuation receipt after exact root registration and before any slot
	// runtime is allowed to start.
	continuationStartAdmission func(context.Context) error
	// requireImmediateStart 禁止持租约调用退入普通用户队列，队列不能携带启动回调。
	requireImmediateStart bool
}

// InterruptRequest 表示 Room 会话中断请求。按 root round + agent slot 定位执行对象。
type InterruptRequest struct {
	SessionKey   string
	RoundID      string
	AgentRoundID string
}

// roomContextStore 是 realtime 读取和更新持久化 Room 状态所需的最小能力集。
type roomContextStore interface {
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
	GetConversationContextForSystem(context.Context, string) (*protocol.ConversationContextAggregate, error)
	UpdateSessionRuntimeIdentity(context.Context, string, string, string) error
	TouchConversationActivity(context.Context, string, time.Time) error
	MarkConversationStarted(context.Context, string, time.Time) error
	BuildRoomSkillPrompt(context.Context, []string) (string, error)
}

// 共用宿主依赖类型统一定义在 runtimehost；以下别名保持本包既有 API 名称。
type (
	MCPServerBuilder                       = runtimehost.MCPServerBuilder
	ConfigurationRuntimeEnvironmentBuilder = runtimehost.ConfigurationRuntimeEnvironmentBuilder
	NexusMCPServerBuilder                  = runtimehost.NexusMCPServerBuilder
	RuntimeSlashExpander                   = runtimehost.RuntimeSlashExpander
)

type Service struct {
	runtimehost.Host
	externalReply      func(context.Context, string, string, string, protocol.Message) error
	externalPrompt     func(context.Context, string, string, string) (string, error)
	externalPermission func(context.Context, string, string, string) (sdkpermission.Handler, error)
	rooms              roomContextStore
	prefs              roomRuntimePreferencesService
	roomHistory        *workspacestore.RoomHistoryStore
	directedMessages   *workspacestore.RoomDirectedMessageStore
	directedWakes      *workspacestore.RoomDirectedMessageWakeStore
	publicHandoffs     *workspacestore.RoomPublicHandoffStore
	goals              goalContextProvider
	factory            roomClientFactory
	broadcaster        RoomBroadcaster
	titles             roomTitleScheduler

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

type goalContextProvider interface {
	RuntimeContext(context.Context, string) (string, *protocol.Goal, error)
	RecordUsageForSession(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	RecordUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error)
	RecordContinuationRuntimeProgress(context.Context, string, goalsvc.ContinuationRuntimeIdentity, bool, ...int64) (*protocol.Goal, error)
	RecordContinuationRuntimeFailure(context.Context, string, goalsvc.ContinuationRuntimeIdentity, string, ...int64) (*protocol.Goal, error)
	RecordContinuationRuntimeCompletionCommandMiss(context.Context, string, goalsvc.ContinuationRuntimeIdentity, string, ...int64) (*protocol.Goal, error)
	RecordGoalActivity(context.Context, string, string, ...int64) (*protocol.Goal, error)
	RecordRoomGoalCollaborationHandback(context.Context, string, string, ...int64) (*protocol.Goal, error)
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

// NewServiceWithFactory 使用非空自定义客户端工厂创建服务；默认实现使用 NewService。
func NewServiceWithFactory(
	cfg config.Config,
	roomService roomContextStore,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
	factory roomClientFactory,
) *Service {
	return &Service{
		rooms:               roomService,
		roomHistory:         workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
		directedMessages:    workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath),
		directedWakes:       workspacestore.NewRoomDirectedMessageWakeStore(cfg.WorkspacePath),
		publicHandoffs:      workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath),
		factory:             factory,
		Host:                runtimehost.NewHost(cfg, agentService, runtimeManager, permission),
		rounds:              newRoomRoundRegistry(),
		goalUsageScopeLocks: newRoomGoalUsageScopeLockRegistry(),
		wakeTimers:          newRoomWakeTimerRegistry(),
	}
}

// SetRoomBroadcaster 注入 Room 共享事件广播器。
func (s *Service) SetRoomBroadcaster(broadcaster RoomBroadcaster) {
	s.broadcaster = broadcaster
	if s.Permission != nil {
		s.Permission.SetRoomBroadcaster(broadcaster)
	}
}

// SetPreferences 注入用户偏好服务，用于 Agent 未显式选模型时读取默认对话模型。
func (s *Service) SetPreferences(prefs roomRuntimePreferencesService) {
	s.prefs = prefs
}

// SetGoalContextProvider 注入 Goal runtime context provider。
func (s *Service) SetGoalContextProvider(provider goalContextProvider) {
	s.goals = provider
}

// SetTitleGenerator 注入会话标题生成器。
func (s *Service) SetTitleGenerator(generator roomTitleScheduler) {
	s.titles = generator
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
	if errs := s.Permission.BroadcastSessionStatus(
		broadcastCtx,
		sessionKey,
		s.Runtime.GetRunningRoundIDs(sessionKey),
	); len(errs) > 0 {
		s.LoggerFor(broadcastCtx).Warn("广播 Room session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *Service) broadcastSharedEvent(ctx context.Context, sessionKey string, roomID string, event protocol.EventMessage) {
	if s.broadcaster != nil && strings.TrimSpace(roomID) != "" {
		s.broadcaster.Broadcast(ctx, roomID, event)
		// RoomBroadcaster 面向房间 WebSocket，不经过 permission.Context；后台自动化需要这条内部镜像。
		s.notifyRoomEventObserver(ctx, sessionKey, event)
		return
	}
	s.Permission.BroadcastEvent(ctx, sessionKey, event)
}

func (s *Service) notifyRoomEventObserver(ctx context.Context, sessionKey string, event protocol.EventMessage) {
	roundID := eventRoundID(event)
	if roundID == "" {
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

// SetReplyPreviewRepository 注入消息落盘后的独立摘要投影。
func (s *Service) SetReplyPreviewRepository(repository *roomrepo.SQLRepository) {
	s.roomHistory.SetReplyPreviewRepository(repository)
}

// PendingAgentInteraction 只暴露成员执行会话的等待事实与变化信号，不暴露审批内容。
func (s *Service) PendingAgentInteraction(conversationID, agentID string) (bool, <-chan struct{}) {
	return s.Permission.PendingRequestState(protocol.BuildRoomAgentSessionKey(conversationID, agentID, "group"))
}
