// INPUT: DM 领域依赖、runtime Manager、持久存储、owner-scoped Slash expander 与宿主签发的隔离 WorkGraph 保存 scope。
// OUTPUT: 可串行处理显式输入、动态 Workflow、隔离内部保存、队列接力和 Goal continuation 的 Service。
// POS: DM 服务装配和共享状态所有者。
package dm

import (
	"context"
	"errors"
	"sync"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
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
// InputSeq 是这次输入的受理游标，同样由聊天入口铸造一次，round marker、chat_ack 与实时广播共用。
type Request struct {
	queuedWorkBinding         *protocol.ExecutionWorkBinding
	queuedReviewBinding       *protocol.ExecutionReviewBinding
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
	InputSeq                  int64
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
	// goalContinuationAuthority is host-only. It is populated only after the
	// Goal service has validated and claimed the exact continuation plan; an
	// ordinary Internal request must remain agent_internal and fail closed.
	goalContinuationAuthority *runtimectx.GoalContinuationAuthority
	// runtimeWake is host-only. Only the DM idle drain sets it when the runtime
	// starts a turn by itself; the round then consumes that turn without a query.
	runtimeWake *runtimeWakeStart
	// hostTrigger 只由主窗口 inbox 为宿主事件设置：内部轮次的用户行仍作为触发分隔条可见。
	hostTrigger bool
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

// EchoPolicy 是 Echo 域交给主窗口的两个判断：一轮结束后何时回头看，到点时是否开口。
// 事件的写入、取消与起轮都由主窗口 inbox 负责，Echo 域不持有任何状态。
type EchoPolicy interface {
	// ScheduleAfter 返回这轮对话结束后的 Echo 到点时间；ok=false 表示不安排（开关关闭）。
	ScheduleAfter(ctx context.Context, ownerUserID string, finishedAt time.Time) (time.Time, bool, error)
	Decide(ctx context.Context, candidate EchoCandidate) (EchoDecision, error)
}

// EchoCandidate 是到点时交给 gate 判断的主窗口。
type EchoCandidate struct {
	OwnerUserID string
	AgentID     string
	SessionKey  string
}

// EchoDecision 三选一：Prompt 非空即开口，DeferUntil 非零即顺延，否则保持沉默。
type EchoDecision struct {
	Prompt     string
	DeferUntil time.Time
	Reason     string
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
	// UserStop marks the user's stop button. Only it writes the durable queue
	// pause gate and counts as a user stop; host control callers (Goal pause,
	// automation, Echo) leave it false so queued input and recovery are kept.
	UserStop bool
}

// 共用宿主依赖类型统一定义在 runtimehost；以下别名保持本包既有 API 名称。
type (
	MCPServerBuilder                       = runtimehost.MCPServerBuilder
	ConfigurationRuntimeEnvironmentBuilder = runtimehost.ConfigurationRuntimeEnvironmentBuilder
	NexusMCPServerBuilder                  = runtimehost.NexusMCPServerBuilder
	RuntimeSlashExpander                   = runtimehost.RuntimeSlashExpander
)

// Service 负责编排 DM 实时链路。
type Service struct {
	runtimehost.Host
	imAutomationPolicy func(context.Context, imdelivery.Source) (*protocol.RuntimeToolPolicy, error)
	imReplies          *imdelivery.Repository
	imReplyValidate    func(context.Context, string, string) error

	roomStore    roomSessionStore
	roomActivity roomConversationActivityStore
	// inbox 只承载主窗口的输入（用户消息与宿主事件），见 main_inbox.go。
	inbox           *inboxstore.Repository
	mainWindowRooms mainWindowRooms
	// inputQueueDispatchMu serializes explicit input, queue handoff, and Goal continuation at the active-check/start boundary.
	inputQueueDispatchMu contextMutex
	// ponytail: one lock is enough for low-volume DM hooks; split per session only if contention is measured.
	inputQueueGuidanceMu      sync.Mutex
	inputQueueGuidancePending map[string][]preparedDMGuidance
	goals                     goalContextProvider
	scopedSessionPolicy       scopedSessionRuntimePolicyProvider
	titles                    titleScheduler
	replies                   ExternalReplyDispatcher
	connectorRuntimeStates    ConnectorRuntimeStateLoader
	connectorPreparationMu    sync.Mutex
	connectorPreparations     map[string]*connectorRuntimePreparation
	connectorPreparationDelay time.Duration
	connectorPreparationRun   func(context.Context, protocol.Session) error
	echo                      EchoPolicy
	// inboxTimer 只按最近的 available_at 唤醒到点的主窗口，见 main_inbox_timer.go。
	inboxTimer *duework.Loop
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
	PairingID      string
	BindingVersion int64
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

type scopedSessionRuntimePolicyProvider interface {
	RuntimeEditorPolicy(context.Context, string, string) (protocol.ScopedSessionRuntimePolicy, bool, error)
}

type goalContextProvider interface {
	RuntimeContext(context.Context, string) (string, *protocol.Goal, error)
	RecordUsageForSession(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	RecordUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error)
	UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error)
	RecordContinuationProgress(context.Context, string, string, bool, ...int64) (*protocol.Goal, error)
	RecordContinuationRuntimeFailure(context.Context, string, goalsvc.ContinuationRuntimeIdentity, string, ...int64) (*protocol.Goal, error)
	RecordContinuationUserStop(context.Context, string, goalsvc.ContinuationRuntimeIdentity, ...int64) (*protocol.Goal, error)
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
		Host:                      runtimehost.NewHost(cfg, agentService, runtimeManager, permission),
		connectorPreparations:     make(map[string]*connectorRuntimePreparation),
		connectorPreparationDelay: defaultConnectorPreparationDelay,
	}
}

// SetScopedSessionRuntimePolicyProvider 注入宿主签发的精确临时 Session 工具面与系统提示。
func (s *Service) SetScopedSessionRuntimePolicyProvider(provider scopedSessionRuntimePolicyProvider) {
	s.scopedSessionPolicy = provider
}

// SetGoalContextProvider 注入 Goal runtime context provider。
func (s *Service) SetGoalContextProvider(provider goalContextProvider) {
	s.goals = provider
}

// SetEchoPolicy 注入 Echo 的到点与开口判断；未注入时主窗口不安排 Echo。
func (s *Service) SetEchoPolicy(policy EchoPolicy) {
	s.echo = policy
}

// SetRoomSessionStore 注入 room 成员会话索引读写能力。
func (s *Service) SetRoomSessionStore(store roomSessionStore) {
	s.roomStore = store
}

// SetRoomConversationActivityStore 注入 Room conversation 草稿消费与活动时间写入能力。
func (s *Service) SetRoomConversationActivityStore(store roomConversationActivityStore) {
	s.roomActivity = store
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
	if errs := s.Permission.BroadcastSessionStatus(ctx, sessionKey, s.Runtime.GetRunningRoundIDs(sessionKey)); len(errs) > 0 {
		s.LoggerFor(ctx).Warn("广播 session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

// SetInboxStore 注入主窗口 inbox 仓储。
func (s *Service) SetInboxStore(store *inboxstore.Repository) {
	s.inbox = store
	s.inboxTimer = s.newMainInboxTimer()
}

// SetReplyPreviewRepository 注入消息落盘后的独立摘要投影。
func (s *Service) SetReplyPreviewRepository(repository *roomrepo.SQLRepository) {
	s.History.SetReplyPreviewRepository(repository)
}
