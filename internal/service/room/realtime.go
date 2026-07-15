// INPUT: Room 服务依赖、runtime 事件与实时会话请求。
// OUTPUT: Room round、队列和共享事件的进程内编排状态。
// POS: Room 实时服务装配与共享状态定义。
package room

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const (
	interruptForceCancelDelay = 150 * time.Millisecond
	roomBroadcastTimeout      = 5 * time.Second
)

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
	return runtimectx.WrapSDKClient(options)
}

// ChatRequest 表示 Room 共享会话的一次聊天请求。
// RoundID / UserMessageID 由后端 mint：WS 入口不填，HandleChat 内部生成；
// 后端内部调用方（automation / mention / queue）可预置 RoundID。
type ChatRequest struct {
	SessionKey            string
	RoomID                string
	ConversationID        string
	AttachmentAgentID     string
	Content               string
	GoalContext           string
	GoalID                string
	GoalObjectiveRevision int64
	Attachments           []protocol.ChatAttachment
	TargetAgentIDs        []string
	ClientRequestID       string
	ClientMessageID       string
	RoundID               string
	UserMessageID         string
	DeliveryPolicy        protocol.ChatDeliveryPolicy
	BroadcastUserMessage  bool
	Internal              bool
	InputOptions          sdkprotocol.OutboundMessageOptions
	PermissionMode        sdkpermission.Mode
	PermissionHandler     sdkpermission.Handler
	EventObserver         RoomEventObserver
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
	agentID string,
	sessionKey string,
	roundID string,
	sourceContextType string,
	sourceContextID string,
	sourceContextLabel string,
	goalObjectiveRevision *atomic.Int64,
) map[string]sdkmcp.ServerConfig

type RealtimeService struct {
	config           config.Config
	rooms            *Service
	agents           *agentsvc.Service
	runtime          *runtimectx.Manager
	permission       *permissionctx.Context
	providers        clientopts.RuntimeConfigResolver
	prefs            roomRuntimePreferencesService
	history          *workspacestore.AgentHistoryStore
	roomHistory      *workspacestore.RoomHistoryStore
	directedMessages *workspacestore.RoomDirectedMessageStore
	directedWakes    *workspacestore.RoomDirectedMessageWakeStore
	inputQueue       *workspacestore.InputQueueStore
	usage            usageRecorder
	quota            quotaChecker
	goals            goalContextProvider
	factory          roomClientFactory
	broadcaster      RoomBroadcaster
	logger           *slog.Logger
	mcpServers       MCPServerBuilder
	titles           roomTitleScheduler

	mu           sync.Mutex
	activeRounds map[string]*activeRoomRound
	guidanceMu   sync.Mutex
	guidance     map[*activeRoomSlot]pendingRoomGuidance
	// ponytail: one global input/round handoff lock; split per conversation only if contention becomes measurable.
	inputQueueDispatchMu sync.Mutex
	// ponytail: one global Agent wake admission lock; split per conversation only if contention becomes measurable.
	publicMentionDispatchMu sync.Mutex
	wakeTimers              *roomWakeTimerRegistry
}

type roomTitleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

type roomRuntimePreferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

type usageRecorder interface {
	RecordMessageUsage(context.Context, usagesvc.RecordInput) error
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
	RecordRoomGoalCollaborationRequired(context.Context, string, string) (*protocol.Goal, error)
	RecordRoomGoalCollaborationEvidence(context.Context, string, string, string, ...int64) (*protocol.Goal, error)
}

type goalContinuationProvider interface {
	PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error)
	GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error)
	ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error)
}

// NewRealtimeService 创建 Room 实时编排服务。
func NewRealtimeService(
	cfg config.Config,
	roomService *Service,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
) *RealtimeService {
	return NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, defaultRoomClientFactory{})
}

// NewRealtimeServiceWithFactory 使用自定义客户端工厂创建服务。
func NewRealtimeServiceWithFactory(
	cfg config.Config,
	roomService *Service,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
	factory roomClientFactory,
) *RealtimeService {
	if factory == nil {
		factory = defaultRoomClientFactory{}
	}
	return &RealtimeService{
		config:           cfg,
		rooms:            roomService,
		agents:           agentService,
		runtime:          runtimeManager,
		permission:       permission,
		history:          workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		roomHistory:      workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
		directedMessages: workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath),
		directedWakes:    workspacestore.NewRoomDirectedMessageWakeStore(cfg.WorkspacePath),
		inputQueue:       workspacestore.NewInputQueueStore(cfg.WorkspacePath),
		factory:          factory,
		logger:           logx.NewDiscardLogger(),
		activeRounds:     make(map[string]*activeRoomRound),
		wakeTimers:       newRoomWakeTimerRegistry(),
	}
}

// SetRoomBroadcaster 注入 Room 共享事件广播器。
func (s *RealtimeService) SetRoomBroadcaster(broadcaster RoomBroadcaster) {
	s.broadcaster = broadcaster
}

// SetLogger 注入业务日志实例。
func (s *RealtimeService) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetProviderResolver 注入 Provider 运行时解析器。
func (s *RealtimeService) SetProviderResolver(resolver clientopts.RuntimeConfigResolver) {
	s.providers = resolver
}

// SetPreferences 注入用户偏好服务，用于 Agent 未显式选模型时读取默认对话模型。
func (s *RealtimeService) SetPreferences(prefs roomRuntimePreferencesService) {
	s.prefs = prefs
}

// SetUsageRecorder 注入 token usage 持久化 ledger。
func (s *RealtimeService) SetUsageRecorder(recorder usageRecorder) {
	s.usage = recorder
}

// SetGoalContextProvider 注入 Goal runtime context provider。
func (s *RealtimeService) SetGoalContextProvider(provider goalContextProvider) {
	s.goals = provider
}

// SetMCPServerBuilder 注入按会话上下文构造 MCP server 的工厂。
func (s *RealtimeService) SetMCPServerBuilder(builder MCPServerBuilder) {
	s.mcpServers = builder
}

// SetTitleGenerator 注入会话标题生成器。
func (s *RealtimeService) SetTitleGenerator(generator roomTitleScheduler) {
	s.titles = generator
}

func (s *RealtimeService) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}
