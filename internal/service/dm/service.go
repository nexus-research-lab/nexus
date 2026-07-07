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
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

var (
	// ErrRoomSessionNotImplemented 表示 Room 请求不应落到 DM service。
	ErrRoomSessionNotImplemented = errors.New("room session must be handled by room service")
)

// Request 表示一次 DM 会话写入请求。
// RoundID / UserMessageID / AgentRoundID 由后端 mint：
// WS 入口不填，HandleChat 内部生成；后端内部调用方（automation / queue / goal）可预置 RoundID。
type Request struct {
	SessionKey             string
	AgentID                string
	Content                string
	GoalContext            string
	HistoryContextPrefix   string
	Attachments            []protocol.ChatAttachment
	ClientRequestID        string
	ClientMessageID        string
	RoundID                string
	UserMessageID          string
	AgentRoundID           string
	DeliveryPolicy         protocol.ChatDeliveryPolicy
	BroadcastUserMessage   bool
	ForceNewRuntimeSession bool
	RewriteTargetRoundID   string
	Internal               bool
	InputOptions           sdkprotocol.OutboundMessageOptions
	PermissionMode         sdkpermission.Mode
	PermissionHandler      sdkpermission.Handler
	ExternalReplyTarget    *ExternalReplyTarget
}

// RewriteRequest 表示一次 DM 最后一条用户消息重写请求。replacement round_id 由后端 mint。
type RewriteRequest struct {
	SessionKey      string
	AgentID         string
	TargetRoundID   string
	ClientRequestID string
	ClientMessageID string
	Content         string
	Attachments     []protocol.ChatAttachment
}

// InterruptRequest 表示一次中断请求。
type InterruptRequest struct {
	SessionKey string
	RoundID    string
}

// MCPServerBuilder 由 server app 注入，按当前会话上下文构造一组 MCP server。
// 用 string 形参避免 dm 包反向依赖 automation 子包，防止 import cycle。
type MCPServerBuilder func(
	agentID string,
	sessionKey string,
	roundID string,
	sourceContextType string,
	sourceContextID string,
	sourceContextLabel string,
) map[string]sdkmcp.ServerConfig

// Service 负责编排 DM 实时链路。
type Service struct {
	config     config.Config
	agents     *agentsvc.Service
	runtime    *runtimectx.Manager
	permission *permissionctx.Context
	roomStore  roomSessionStore
	providers  clientopts.RuntimeConfigResolver
	prefs      runtimePreferencesService
	files      *workspacestore.SessionFileStore
	history    *workspacestore.AgentHistoryStore
	inputQueue *workspacestore.InputQueueStore
	usage      usageRecorder
	goals      goalContextProvider
	logger     *slog.Logger
	mcpServers MCPServerBuilder
	titles     titleScheduler
	replies    ExternalReplyDispatcher
}

// ExternalReplyTarget 是 DM 完成后回送外部 IM 通道的最小目标描述。
type ExternalReplyTarget struct {
	Mode       string
	Channel    string
	To         string
	AccountID  string
	ThreadID   string
	SessionKey string
}

// ExternalReplyResult 是外部 IM 回投后的最小可观测结果。
type ExternalReplyResult struct {
	Channel                  string
	To                       string
	ThreadID                 string
	PrimaryPlatformMessageID string
	PlatformMessageIDs       []string
}

// ExternalReplyDispatcher 由 app 装配层注入，避免 dm 包反向依赖 channels。
type ExternalReplyDispatcher interface {
	DeliverExternalReply(context.Context, string, string, ExternalReplyTarget) (ExternalReplyResult, error)
	SetExternalTyping(context.Context, string, ExternalReplyTarget, bool) error
}

type roomSessionStore interface {
	GetRoomSessionByKey(context.Context, string, protocol.SessionKey) (*protocol.Session, error)
	UpdateRoomSessionSDKSessionID(context.Context, string, string) error
}

type titleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

type runtimePreferencesService interface {
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
	RecordContinuationProgress(context.Context, string, string, bool) (*protocol.Goal, error)
	RecordContinuationFailure(context.Context, string, string, string) (*protocol.Goal, error)
	RecordCompletionToolMiss(context.Context, string, string, string) (*protocol.Goal, error)
	RecordGoalActivity(context.Context, string, string) (*protocol.Goal, error)
	PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error)
	GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error)
}

type goalContinuationPlanReleaser interface {
	ReleaseContinuationPlan(context.Context, protocol.GoalContinuation, string) (*protocol.Goal, error)
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

// SetMCPServerBuilder 注入按会话上下文构造 MCP server 的工厂。
// 由 server app 在构造定时任务服务后注入，避免 dm 包反向依赖 automation 子包。
func (s *Service) SetMCPServerBuilder(builder MCPServerBuilder) {
	s.mcpServers = builder
}

// SetProviderResolver 注入 Provider 运行时解析器。
func (s *Service) SetProviderResolver(resolver clientopts.RuntimeConfigResolver) {
	s.providers = resolver
}

// SetPreferences 注入用户偏好服务，用于 Agent 未显式选模型时读取默认对话模型。
func (s *Service) SetPreferences(prefs runtimePreferencesService) {
	s.prefs = prefs
}

// SetUsageRecorder 注入 token usage 持久化 ledger。
func (s *Service) SetUsageRecorder(recorder usageRecorder) {
	s.usage = recorder
}

// SetGoalContextProvider 注入 Goal runtime context provider。
func (s *Service) SetGoalContextProvider(provider goalContextProvider) {
	s.goals = provider
}

// SetRoomSessionStore 注入 room 成员会话索引读写能力。
func (s *Service) SetRoomSessionStore(store roomSessionStore) {
	s.roomStore = store
}

// SetTitleGenerator 注入会话标题生成器。
func (s *Service) SetTitleGenerator(generator titleScheduler) {
	s.titles = generator
}

// SetExternalReplyDispatcher 注入外部 IM 自然回复投递器。
func (s *Service) SetExternalReplyDispatcher(dispatcher ExternalReplyDispatcher) {
	s.replies = dispatcher
}

func (s *Service) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	if errs := s.permission.BroadcastSessionStatus(ctx, sessionKey, s.runtime.GetRunningRoundIDs(sessionKey)); len(errs) > 0 {
		s.loggerFor(ctx).Warn("广播 session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}
