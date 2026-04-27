package dm

import (
	"context"
	"errors"
	"log/slog"

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

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

var (
	// ErrRoomSessionNotImplemented 表示 Room 请求不应落到 DM service。
	ErrRoomSessionNotImplemented = errors.New("room session must be handled by room service")
)

// Request 表示一次 DM 会话写入请求。
type Request struct {
	SessionKey           string
	AgentID              string
	Content              string
	RoundID              string
	ReqID                string
	DeliveryPolicy       protocol.ChatDeliveryPolicy
	BroadcastUserMessage bool
	PermissionMode       sdkprotocol.PermissionMode
	PermissionHandler    agentclient.PermissionHandler
}

// InterruptRequest 表示一次中断请求。
type InterruptRequest struct {
	SessionKey string
	RoundID    string
}

// MCPServerBuilder 由 server app 注入，按当前会话上下文构造一组进程内 MCP server。
// 用 string 形参避免 dm 包反向依赖 automation 子包，防止 import cycle。
type MCPServerBuilder func(agentID, sessionKey, sourceContextType string) map[string]agentclient.SDKMCPServer

// Service 负责编排 DM 实时链路。
type Service struct {
	config     config.Config
	agents     *agentsvc.Service
	runtime    *runtimectx.Manager
	permission *permissionctx.Context
	roomStore  roomSessionStore
	providers  clientopts.RuntimeConfigResolver
	files      *workspacestore.SessionFileStore
	history    *workspacestore.AgentHistoryStore
	inputQueue *workspacestore.InputQueueStore
	usage      usageRecorder
	logger     *slog.Logger
	mcpServers MCPServerBuilder
	titles     titleScheduler
}

type roomSessionStore interface {
	GetRoomSessionByKey(context.Context, string, protocol.SessionKey) (*protocol.Session, error)
	UpdateRoomSessionSDKSessionID(context.Context, string, string) error
}

type titleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

type usageRecorder interface {
	RecordMessageUsage(context.Context, usagesvc.RecordInput) error
}

// NewService 创建 DM 会话编排服务。
func NewService(
	cfg config.Config,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
) *Service {
	return &Service{
		config:     cfg,
		agents:     agentService,
		runtime:    runtimeManager,
		permission: permission,
		files:      workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		inputQueue: workspacestore.NewInputQueueStore(cfg.WorkspacePath),
		logger:     logx.NewDiscardLogger(),
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

// SetMCPServerBuilder 注入按会话上下文构造进程内 MCP server 的工厂。
// 由 server app 在构造定时任务服务后注入，避免 dm 包反向依赖 automation 子包。
func (s *Service) SetMCPServerBuilder(builder MCPServerBuilder) {
	s.mcpServers = builder
}

// SetProviderResolver 注入 Provider 运行时解析器。
func (s *Service) SetProviderResolver(resolver clientopts.RuntimeConfigResolver) {
	s.providers = resolver
}

// SetUsageRecorder 注入 token usage 持久化 ledger。
func (s *Service) SetUsageRecorder(recorder usageRecorder) {
	s.usage = recorder
}

// SetRoomSessionStore 注入 room 成员会话索引读写能力。
func (s *Service) SetRoomSessionStore(store roomSessionStore) {
	s.roomStore = store
}

// SetTitleGenerator 注入会话标题生成器。
func (s *Service) SetTitleGenerator(generator titleScheduler) {
	s.titles = generator
}

func (s *Service) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	if errs := s.permission.BroadcastSessionStatus(ctx, sessionKey, s.runtime.GetRunningRoundIDs(sessionKey)); len(errs) > 0 {
		s.loggerFor(ctx).Warn("广播 session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}
