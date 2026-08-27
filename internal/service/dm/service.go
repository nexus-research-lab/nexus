// INPUT: DM 领域依赖、runtime Manager、持久存储、owner-scoped Slash expander 与宿主签发的隔离 WorkGraph 保存 scope。
// OUTPUT: 可串行处理显式输入、动态 Workflow、隔离内部保存、队列接力和 Goal continuation 的 Service。
// POS: DM 服务装配和共享状态所有者。
package dm

import (
	"context"
	"errors"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
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
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

var (
	// ErrRoomSessionNotImplemented 表示 Room 请求不应落到 DM service。
	ErrRoomSessionNotImplemented = errors.New("room session must be handled by room service")
)

// contextMutex 让等待串行边界的后台任务能在 session 关闭时响应取消。
// Lock/Unlock 仅保留给不带 context 的短临界区与并发测试。
type contextMutex struct {
	once  sync.Once
	token chan struct{}
}

func (m *contextMutex) initialize() {
	m.once.Do(func() {
		m.token = make(chan struct{}, 1)
		m.token <- struct{}{}
	})
}

func (m *contextMutex) Lock() {
	_ = m.LockContext(context.Background())
}

func (m *contextMutex) LockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.initialize()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.token:
		return nil
	}
}

func (m *contextMutex) Unlock() {
	m.initialize()
	select {
	case m.token <- struct{}{}:
	default:
		panic("unlock of unlocked context mutex")
	}
}

// Request 表示一次 DM 会话写入请求。
// RoundID / UserMessageID / AgentRoundID 由后端 mint：
// WS 入口不填，由聊天入口生成；后端内部调用方（automation / queue / goal）可预置 RoundID。
type Request struct {
	SessionKey                string
	AgentID                   string
	Content                   string
	GoalContext               string
	GoalID                    string
	GoalObjectiveRevision     int64
	ExecutionID               string
	Attachments               []protocol.ChatAttachment
	ClientRequestID           string
	ClientMessageID           string
	RoundID                   string
	UserMessageID             string
	AgentRoundID              string
	DeliveryPolicy            protocol.ChatDeliveryPolicy
	BroadcastUserMessage      bool
	RewriteTargetRoundID      string
	RewriteRemoveMessageUUIDs []string
	RewriteRemoveRoundIDs     []string
	RewriteRemoveMessageCount int
	Internal                  bool
	// TrustedConfigurationContext 仅由 Nexus WebSocket 用户入口设置，后台/外部/队列不得继承。
	TrustedConfigurationContext bool
	// ExecutionOrigin 由服务端调度器写入；非空值不会获得持久配置 capability。
	ExecutionOrigin string
	// WorkGraphSaveSourceSessionKey 只由宿主的 WorkGraph 保存调度器写入，
	// 让隔离内部 Session 的 exact CLI capability 绑定原 preview 来源而不续写其 transcript。
	WorkGraphSaveSourceSessionKey string
	// TrustedExternalInteractiveContext 仅由 channels ingress 在实时复核 active
	// pairing 后设置；只提升同 Agent 私聊能力，不授予 owner-wide 控制面权限。
	TrustedExternalInteractiveContext bool
	// trustedQueuedConfigurationContext 只能由本包在成功 claim 宿主 DB
	// admission 后设置，外部 Request 构造者无法伪造。
	trustedQueuedConfigurationContext bool
	// rewriteOverlayOnly 只能由本包 rewrite planner 在确认目标是未进入 SDK
	// transcript 的 durable 失败 round 后设置，外部 Request 无法绕过 UUID 边界。
	rewriteOverlayOnly bool
	// forkSourceSessionID / forkMessageID 只由 Room service 写入，HTTP/WS 请求不能伪造。
	forkSourceSessionID string
	forkMessageID       string
	// runtimePreparationOnly 只由 Session 设置提交后的后台预备器设置；它只允许
	// 物化工具面 fork，不创建用户消息或模型 round。
	runtimePreparationOnly       bool
	expectedConfigurationVersion int64
	expectedConnectorSelection   *protocol.SessionConnectorSelection
	InputOptions                 sdkprotocol.OutboundMessageOptions
	PermissionMode               sdkpermission.Mode
	PermissionHandler            sdkpermission.Handler
	// RuntimeToolPolicy 仅供 automation 等受控执行传入创建时权限快照。
	// 普通会话为 nil，继续使用 Agent 当前工具配置。
	RuntimeToolPolicy *protocol.RuntimeToolPolicy
	// AutomationRun 只由 Automation 调度器签发，作为 runtime/MCP 的可信 run 身份。
	AutomationRun       *protocol.AutomationRunContext
	ExternalReplyTarget *ExternalReplyTarget
	// StartAdmission 在 runtime 注册后、模型请求前执行最终后台准入。
	StartAdmission func(context.Context) error
	// DeferredAssistant 让后台 round 只在最终准入通过后写入 assistant 消息。
	DeferredAssistant *DeferredAssistantHooks
	// continuationStartAdmission is host-only. It advances the durable
	// continuation receipt after exact runtime registration and before the
	// provider receives a query.
	continuationStartAdmission func(context.Context) error
}

// DeferredAssistantCandidate 是后台 round 提交前的最终 assistant 快照。
type DeferredAssistantCandidate struct {
	Message        protocol.Message
	TerminalStatus string
	ResultSubtype  string
	ErrorMessage   string
}

// DeferredAssistantOutcome 描述后台消息最终是否成功写入。
type DeferredAssistantOutcome struct {
	Status string
	Error  error
}

// DeferredAssistantHooks 控制后台 assistant 的最终准入与尝试收口。
type DeferredAssistantHooks struct {
	Admit    func(context.Context, DeferredAssistantCandidate) (protocol.Message, bool, error)
	Complete func(context.Context, DeferredAssistantOutcome)
}

// EchoTerminalRound 是 Echo 需要观察的最小成功 DM 终态。
type EchoTerminalRound struct {
	OwnerUserID string
	AgentID     string
	SessionKey  string
	RoundID     string
	AssistantID string
	FinishedAt  time.Time
}

// EchoLifecycleHooks 把普通 DM 用户活动与成功终态交给 Echo 域。
type EchoLifecycleHooks struct {
	OnUserActivity func(context.Context, string, string) ([]string, error)
	OnTerminal     func(context.Context, EchoTerminalRound)
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

// ConfigurationRuntimeEnvironmentBuilder 由宿主为当前 runtime round 签发 nexuscfg 环境。
type ConfigurationRuntimeEnvironmentBuilder func(
	context.Context,
	*protocol.Agent,
	string,
	string,
	string,
	string,
) (map[string]string, error)

// NexusMCPServerBuilder 为当前 physical round 构造唯一 Nexus 内建 MCP server。
type NexusMCPServerBuilder func(
	context.Context,
	nexusmcp.RoundContext,
) (map[string]sdkmcp.ServerConfig, error)

// RuntimeSlashExpander 把 Nexus 产品 Slash 或 owner 的命名 WorkGraph 沉淀展开为 runtime prompt。
type RuntimeSlashExpander interface {
	ExpandRuntimePrompt(context.Context, string, string) (string, error)
}

// Service 负责编排 DM 实时链路。
type Service struct {
	config       config.Config
	agents       *agentsvc.Service
	runtime      *runtimectx.Manager
	permission   *permissionctx.Context
	roomStore    roomSessionStore
	roomActivity roomConversationActivityStore
	providers    clientopts.RuntimeConfigResolver
	admission    clientopts.AgentRuntimeAdmissionResolver
	prefs        runtimePreferencesService
	files        *workspacestore.SessionFileStore
	history      *workspacestore.AgentHistoryStore
	inputQueue   *workspacestore.InputQueueStore
	queueTrust   queueAdmissionStore
	// inputQueueDispatchMu serializes explicit input, queue handoff, and Goal continuation at the active-check/start boundary.
	inputQueueDispatchMu contextMutex
	// ponytail: one lock is enough for low-volume DM hooks; split per session only if contention is measured.
	inputQueueGuidanceMu      sync.Mutex
	inputQueueGuidancePending map[string][]preparedDMGuidance
	usage                     usageRecorder
	quota                     quotaChecker
	goals                     goalContextProvider
	executionContext          executionContextProvider
	subagentAdmission         orchestrationruntimehook.Provider
	logger                    *slog.Logger
	mcpServers                MCPServerBuilder
	configurationRuntimeEnv   ConfigurationRuntimeEnvironmentBuilder
	nexusMCP                  NexusMCPServerBuilder
	runtimeSlashExpander      RuntimeSlashExpander
	scopedSessionPolicy       scopedSessionRuntimePolicyProvider
	titles                    titleScheduler
	replies                   ExternalReplyDispatcher
	connectorRuntimeStates    ConnectorRuntimeStateLoader
	connectorPreparationMu    sync.Mutex
	connectorPreparations     map[string]*connectorRuntimePreparation
	connectorPreparationDelay time.Duration
	connectorPreparationRun   func(context.Context, protocol.Session) error
	echoHooks                 EchoLifecycleHooks
}

// ConnectorRuntimeState 是可安全进入模型动态上下文的 Connector 脱敏状态。
type ConnectorRuntimeState struct {
	ConnectorID string
	Configured  bool
}

// ConnectorRuntimeStateLoader 读取 owner 的 Connector 配置/授权状态，不返回凭据。
type ConnectorRuntimeStateLoader func(context.Context, string) ([]ConnectorRuntimeState, error)

// ExternalReplyTarget 是 DM 完成后回送外部 IM 通道的最小目标描述。
type ExternalReplyTarget struct {
	Mode           string
	Channel        string
	To             string
	AccountID      string
	ThreadID       string
	SessionKey     string
	ContextToken   string
	ReplyContextID string
	StreamID       string
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
	UpdateRoomSessionRuntimeIdentity(context.Context, string, string, string) error
}

type roomConversationActivityStore interface {
	MarkConversationStarted(context.Context, string, time.Time) error
}

type titleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

type runtimePreferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

type scopedSessionRuntimePolicyProvider interface {
	RuntimeEditorPolicy(string, string) (protocol.ScopedSessionRuntimePolicy, bool, error)
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

type goalContextProvider interface {
	RuntimeContext(context.Context, string) (string, *protocol.Goal, error)
	RecordUsageForSession(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	RecordUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error)
	RecordContinuationProgress(context.Context, string, string, bool, ...int64) (*protocol.Goal, error)
	RecordContinuationFailure(context.Context, string, string, string, ...int64) (*protocol.Goal, error)
	RecordCompletionCommandMiss(context.Context, string, string, string, ...int64) (*protocol.Goal, error)
	RecordGoalActivity(context.Context, string, string, ...int64) (*protocol.Goal, error)
	PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error)
	GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error)
	ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error)
}

// NewService 创建 DM 会话编排服务。
func NewService(
	cfg config.Config,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
) *Service {
	return &Service{
		config:                    cfg,
		agents:                    agentService,
		runtime:                   runtimeManager,
		permission:                permission,
		files:                     workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:                   workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		inputQueue:                workspacestore.NewInputQueueStore(cfg.WorkspacePath),
		logger:                    logx.NewDiscardLogger(),
		connectorPreparations:     make(map[string]*connectorRuntimePreparation),
		connectorPreparationDelay: defaultConnectorPreparationDelay,
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

// SetConfigurationRuntimeEnvironmentBuilder 注入可信 nexuscfg capability 签发器。
func (s *Service) SetConfigurationRuntimeEnvironmentBuilder(
	builder ConfigurationRuntimeEnvironmentBuilder,
) {
	s.configurationRuntimeEnv = builder
}

// SetNexusMCPServerBuilder 注入可信的 Nexus 内建 MCP server 工厂。
func (s *Service) SetNexusMCPServerBuilder(
	builder NexusMCPServerBuilder,
) {
	s.nexusMCP = builder
}

// SetRuntimeSlashExpander 注入 owner-scoped WorkGraph 沉淀 prompt 展开器。
func (s *Service) SetRuntimeSlashExpander(expander RuntimeSlashExpander) {
	s.runtimeSlashExpander = expander
}

// SetScopedSessionRuntimePolicyProvider 注入宿主签发的精确临时 Session 工具面与系统提示。
func (s *Service) SetScopedSessionRuntimePolicyProvider(provider scopedSessionRuntimePolicyProvider) {
	s.scopedSessionPolicy = provider
}

// SetSubagentAdmissionProvider 注入 Agent tool 的权威 WorkGraph 准入与 Attempt lifecycle。
func (s *Service) SetSubagentAdmissionProvider(provider orchestrationruntimehook.Provider) {
	s.subagentAdmission = provider
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

// SetEchoLifecycleHooks 注入 Echo 的取消与调度回调。
func (s *Service) SetEchoLifecycleHooks(hooks EchoLifecycleHooks) {
	s.echoHooks = hooks
}

// SetRoomSessionStore 注入 room 成员会话索引读写能力。
func (s *Service) SetRoomSessionStore(store roomSessionStore) {
	s.roomStore = store
}

// SetRoomConversationActivityStore 注入 Room conversation 草稿消费与活动时间写入能力。
func (s *Service) SetRoomConversationActivityStore(store roomConversationActivityStore) {
	s.roomActivity = store
}

// SetQueueAdmissionStore 注入宿主 DB 中不可由 Agent workspace 伪造的队列信任根。
func (s *Service) SetQueueAdmissionStore(store queueAdmissionStore) {
	s.queueTrust = store
}

// SetTitleGenerator 注入会话标题生成器。
func (s *Service) SetTitleGenerator(generator titleScheduler) {
	s.titles = generator
}

// SetExternalReplyDispatcher 注入外部 IM 自然回复投递器。
func (s *Service) SetExternalReplyDispatcher(dispatcher ExternalReplyDispatcher) {
	s.replies = dispatcher
}

// SetConnectorRuntimeStateLoader 注入 Connector 配置/授权状态真相源。
func (s *Service) SetConnectorRuntimeStateLoader(loader ConnectorRuntimeStateLoader) {
	s.connectorRuntimeStates = loader
}

func (s *Service) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	if errs := s.permission.BroadcastSessionStatus(ctx, sessionKey, s.runtime.GetRunningRoundIDs(sessionKey)); len(errs) > 0 {
		s.loggerFor(ctx).Warn("广播 session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}
