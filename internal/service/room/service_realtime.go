package room

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
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

type defaultRoomClientFactory struct{}

func (f defaultRoomClientFactory) New(options agentclient.Options) runtimectx.Client {
	return runtimectx.WrapSDKClient(options)
}

// ChatRequest 表示 Room 共享会话的一次聊天请求。
type ChatRequest struct {
	SessionKey           string
	RoomID               string
	ConversationID       string
	AttachmentAgentID    string
	Content              string
	Attachments          []protocol.ChatAttachment
	RoundID              string
	ReqID                string
	DeliveryPolicy       protocol.ChatDeliveryPolicy
	BroadcastUserMessage bool
	Internal             bool
	InputOptions         sdkprotocol.OutboundMessageOptions
}

// InterruptRequest 表示 Room 会话中断请求。
type InterruptRequest struct {
	SessionKey string
	MsgID      string
}

// MCPServerBuilder 由 server app 注入，按当前会话上下文构造一组进程内 MCP server。
// 用 string 形参避免 room domain 反向依赖 automation 子包，防止 import cycle。
type MCPServerBuilder func(
	agentID string,
	sessionKey string,
	sourceContextType string,
	sourceContextID string,
	sourceContextLabel string,
) map[string]sdkmcp.SDKMCPServer

type RealtimeService struct {
	config      config.Config
	rooms       *Service
	agents      *agentsvc.Service
	runtime     *runtimectx.Manager
	permission  *permissionctx.Context
	providers   clientopts.RuntimeConfigResolver
	history     *workspacestore.AgentHistoryStore
	roomHistory *workspacestore.RoomHistoryStore
	actions     *workspacestore.RoomActionStore
	inputQueue  *workspacestore.InputQueueStore
	usage       usageRecorder
	goals       goalContextProvider
	factory     roomClientFactory
	broadcaster RoomBroadcaster
	logger      *slog.Logger
	mcpServers  MCPServerBuilder
	titles      roomTitleScheduler
	internalAPI roomInternalAPI

	mu           sync.Mutex
	activeRounds map[string]*activeRoomRound
}

type roomInternalAPI struct {
	BaseURL string
	Token   string
}

type roomTitleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

type usageRecorder interface {
	RecordMessageUsage(context.Context, usagesvc.RecordInput) error
}

type goalContextProvider interface {
	RuntimeContext(context.Context, string) (string, *protocol.Goal, error)
	RecordUsageForSession(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	RecordUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error)
	RecordContinuationProgress(context.Context, string, string, bool) (*protocol.Goal, error)
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
		config:       cfg,
		rooms:        roomService,
		agents:       agentService,
		runtime:      runtimeManager,
		permission:   permission,
		history:      workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		roomHistory:  workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
		actions:      workspacestore.NewRoomActionStore(cfg.WorkspacePath),
		inputQueue:   workspacestore.NewInputQueueStore(cfg.WorkspacePath),
		factory:      factory,
		logger:       logx.NewDiscardLogger(),
		activeRounds: make(map[string]*activeRoomRound),
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

// SetUsageRecorder 注入 token usage 持久化 ledger。
func (s *RealtimeService) SetUsageRecorder(recorder usageRecorder) {
	s.usage = recorder
}

// SetGoalContextProvider 注入 Goal runtime context provider。
func (s *RealtimeService) SetGoalContextProvider(provider goalContextProvider) {
	s.goals = provider
}

// SetMCPServerBuilder 注入按会话上下文构造进程内 MCP server 的工厂。
func (s *RealtimeService) SetMCPServerBuilder(builder MCPServerBuilder) {
	s.mcpServers = builder
}

// SetTitleGenerator 注入会话标题生成器。
func (s *RealtimeService) SetTitleGenerator(generator roomTitleScheduler) {
	s.titles = generator
}

// SetInternalAPI 注入 Room runtime 触发 nexusctl 时访问常驻 server 的内部控制面配置。
func (s *RealtimeService) SetInternalAPI(baseURL string, token string) {
	s.internalAPI = roomInternalAPI{
		BaseURL: strings.TrimSpace(baseURL),
		Token:   strings.TrimSpace(token),
	}
}

func (s *RealtimeService) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}
