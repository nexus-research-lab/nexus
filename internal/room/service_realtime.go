package room

import (
	"context"
	"log/slog"
	"sync"
	"time"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/conversation/titlegen"
	"github.com/nexus-research-lab/nexus/internal/logx"
	permission3 "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

const (
	interruptForceCancelDelay = 150 * time.Millisecond
	roomBroadcastTimeout      = 5 * time.Second
)

type roomClientFactory interface {
	New(agentclient.Options) runtimectx.Client
}

// RoomBroadcaster 负责把 Room 共享事件扇出到 room 级订阅者。
type RoomBroadcaster interface {
	Broadcast(context.Context, string, protocol.EventMessage) []error
}

type defaultRoomClientFactory struct{}

func (f defaultRoomClientFactory) New(options agentclient.Options) runtimectx.Client {
	return runtimectx.WrapSDKClient(agentclient.New(options))
}

// ChatRequest 表示 Room 共享会话的一次聊天请求。
type ChatRequest struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	Content        string
	RoundID        string
	ReqID          string
}

// InterruptRequest 表示 Room 会话中断请求。
type InterruptRequest struct {
	SessionKey string
	MsgID      string
}

// RealtimeService 负责 Room 的共享流实时编排。
// MCPServerBuilder 由 bootstrap 注入，按当前会话上下文构造一组进程内 MCP server。
// 用 string 形参避免 room 包反向依赖 automation 子包，防止 import cycle。
type MCPServerBuilder func(agentID, sessionKey, sourceContextType string) map[string]agentclient.SDKMCPServer

type RealtimeService struct {
	config      config.Config
	rooms       *Service
	agents      *agent2.Service
	runtime     *runtimectx.Manager
	permission  *permission3.Context
	providers   runtimectx.RuntimeConfigResolver
	history     *workspacestore.AgentHistoryStore
	roomHistory *workspacestore.RoomHistoryStore
	factory     roomClientFactory
	broadcaster RoomBroadcaster
	logger      *slog.Logger
	mcpServers  MCPServerBuilder
	titles      roomTitleScheduler

	mu           sync.Mutex
	activeRounds map[string]*activeRoomRound
}

type roomTitleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

// NewRealtimeService 创建 Room 实时编排服务。
func NewRealtimeService(
	cfg config.Config,
	roomService *Service,
	agentService *agent2.Service,
	runtimeManager *runtimectx.Manager,
	permission *permission3.Context,
) *RealtimeService {
	return NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, defaultRoomClientFactory{})
}

// NewRealtimeServiceWithFactory 使用自定义客户端工厂创建服务。
func NewRealtimeServiceWithFactory(
	cfg config.Config,
	roomService *Service,
	agentService *agent2.Service,
	runtimeManager *runtimectx.Manager,
	permission *permission3.Context,
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
func (s *RealtimeService) SetProviderResolver(resolver runtimectx.RuntimeConfigResolver) {
	s.providers = resolver
}

// SetMCPServerBuilder 注入按会话上下文构造进程内 MCP server 的工厂。
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
