// INPUT: app 装配注入的 Provider、admission、队列信任、用量、额度、执行上下文与 MCP 工厂。
// OUTPUT: DM 与 Room realtime 共用的依赖与运行阶段。
// POS: 两个会话宿主的共享骨架；会话拓扑与公私域策略仍归各自宿主。
package runtimehost

import (
	"context"
	"log/slog"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// MCPServerBuilder 由 server app 注入，按当前会话上下文构造一组 MCP server。
// 用 string 形参避免会话宿主反向依赖 automation 子包，防止 import cycle。
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

// QueueAdmissionStore 持久化可信队列受理与一次性领取。
type QueueAdmissionStore interface {
	Record(context.Context, queueadmissionstore.Admission) error
	Claim(context.Context, queueadmissionstore.Binding) (queueadmissionstore.Claim, bool, error)
	Release(context.Context, queueadmissionstore.Claim) error
	Consume(context.Context, queueadmissionstore.Claim) error
	Revoke(context.Context, queueadmissionstore.Binding) error
}

// UsageRecorder 持久化 token usage ledger。
type UsageRecorder interface {
	RecordMessageUsage(context.Context, usagesvc.RecordInput) error
}

// QuotaChecker 在新 runtime 请求前按 owner entitlement 校验额度。
type QuotaChecker interface {
	EnsureQuotaAvailable(context.Context, string) error
}

// ExecutionContextProvider 读取每轮权威 WorkGraph 上下文。
type ExecutionContextProvider interface {
	RuntimeContext(context.Context, orchestrationsvc.ActorContext) (string, error)
}

// Host 是 DM 与 Room realtime 共用的宿主依赖集合；两者的 Service 嵌入它。
type Host struct {
	Config     config.Config
	Agents     *agentsvc.Service
	Runtime    *runtimectx.Manager
	Permission *permissionctx.Context
	Files      *workspacestore.SessionFileStore
	History    *workspacestore.AgentHistoryStore
	InputQueue *workspacestore.InputQueueStore

	Providers               clientopts.RuntimeConfigResolver
	Admission               clientopts.AgentRuntimeAdmissionResolver
	QueueTrust              QueueAdmissionStore
	Usage                   UsageRecorder
	Quota                   QuotaChecker
	ExecutionContext        ExecutionContextProvider
	SubagentAdmission       orchestrationruntimehook.Provider
	Logger                  *slog.Logger
	MCPServers              MCPServerBuilder
	ConfigurationRuntimeEnv ConfigurationRuntimeEnvironmentBuilder
	NexusMCP                NexusMCPServerBuilder
	RuntimeSlashExpander    RuntimeSlashExpander
}

// NewHost 创建会话宿主共用的基础依赖与 workspace 存储；其余依赖由 app 装配注入。
func NewHost(
	cfg config.Config,
	agentService *agentsvc.Service,
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
) Host {
	return Host{
		Config:     cfg,
		Agents:     agentService,
		Runtime:    runtimeManager,
		Permission: permission,
		Files:      workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		History:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		InputQueue: workspacestore.NewInputQueueStore(cfg.WorkspacePath),
		Logger:     logx.NewDiscardLogger(),
	}
}

// SetLogger 注入业务日志实例。
func (h *Host) SetLogger(logger *slog.Logger) {
	if logger == nil {
		h.Logger = logx.NewDiscardLogger()
		return
	}
	h.Logger = logger
}

// SetProviderResolver 注入 Provider 运行时解析器。
func (h *Host) SetProviderResolver(resolver clientopts.RuntimeConfigResolver) {
	h.Providers = resolver
}

// SetRuntimeAdmissionResolver 注入认证转场与动态强隔离 admission。
func (h *Host) SetRuntimeAdmissionResolver(resolver clientopts.AgentRuntimeAdmissionResolver) {
	h.Admission = resolver
}

// SetQueueAdmissionStore 注入可信队列受理存储。
func (h *Host) SetQueueAdmissionStore(store QueueAdmissionStore) {
	h.QueueTrust = store
}

// SetUsageRecorder 注入 token usage 持久化 ledger。
func (h *Host) SetUsageRecorder(recorder UsageRecorder) {
	h.Usage = recorder
}

// SetQuotaChecker 注入订阅额度检查器。
func (h *Host) SetQuotaChecker(checker QuotaChecker) {
	h.Quota = checker
}

// SetExecutionContextProvider 注入每轮权威 WorkGraph 上下文读取器。
func (h *Host) SetExecutionContextProvider(provider ExecutionContextProvider) {
	h.ExecutionContext = provider
}

// SetSubagentAdmissionProvider 注入 Agent tool 的权威 WorkGraph 准入与 Attempt lifecycle。
func (h *Host) SetSubagentAdmissionProvider(provider orchestrationruntimehook.Provider) {
	h.SubagentAdmission = provider
}

// SetMCPServerBuilder 注入按会话上下文构造 MCP server 的工厂。
func (h *Host) SetMCPServerBuilder(builder MCPServerBuilder) {
	h.MCPServers = builder
}

// SetConfigurationRuntimeEnvironmentBuilder 注入可信 nexuscfg capability 签发器。
func (h *Host) SetConfigurationRuntimeEnvironmentBuilder(builder ConfigurationRuntimeEnvironmentBuilder) {
	h.ConfigurationRuntimeEnv = builder
}

// SetNexusMCPServerBuilder 注入可信的 Nexus 内建 MCP server 工厂。
func (h *Host) SetNexusMCPServerBuilder(builder NexusMCPServerBuilder) {
	h.NexusMCP = builder
}

// SetRuntimeSlashExpander 注入 owner-scoped WorkGraph 沉淀 prompt 展开器。
func (h *Host) SetRuntimeSlashExpander(expander RuntimeSlashExpander) {
	h.RuntimeSlashExpander = expander
}

// LoggerFor 返回绑定请求上下文字段的业务日志。
func (h *Host) LoggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, h.Logger)
}

// EnsureQuotaAvailable 在启动新 runtime 请求前校验当前 owner 额度；未装配时放行。
func (h *Host) EnsureQuotaAvailable(ctx context.Context) error {
	if h.Quota == nil {
		return nil
	}
	return h.Quota.EnsureQuotaAvailable(ctx, authctx.OwnerUserID(ctx))
}

// ExecutionContextualInputs 读取 actor 的权威 WorkGraph 上下文并投影为下一轮隐藏输入。
func (h *Host) ExecutionContextualInputs(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
) ([]runtimectx.ContextualInputBlock, error) {
	if h.ExecutionContext == nil {
		return nil, nil
	}
	content, err := h.ExecutionContext.RuntimeContext(ctx, actor)
	if err != nil {
		return nil, err
	}
	return runtimectx.ExecutionContextualInputs(content), nil
}

// ExecutionObserver 返回统一的 DM/Room 运行观察器。
func (h *Host) ExecutionObserver() orchestrationruntimehook.Observer {
	return orchestrationruntimehook.Observer{Provider: h.ExecutionContext, Logger: h.Logger}
}

// ExpandRuntimeSlashPrompt 展开产品 Slash 或 owner 命名 WorkGraph 的 runtime prompt。
func (h *Host) ExpandRuntimeSlashPrompt(ctx context.Context, content string) (string, error) {
	if h.RuntimeSlashExpander != nil {
		return h.RuntimeSlashExpander.ExpandRuntimePrompt(ctx, authctx.OwnerUserID(ctx), content)
	}
	return slashcommandsvc.ExpandProductPrompt(content), nil
}

type imagegenDefaultResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

// RuntimeImagegenDefaultEnabled 判断 owner 默认生图配置是否可用。
func (h *Host) RuntimeImagegenDefaultEnabled(ctx context.Context) bool {
	resolver, ok := h.Providers.(imagegenDefaultResolver)
	if !ok || resolver == nil {
		return false
	}
	_, err := resolver.ResolveImageConfig(ctx, "")
	return err == nil
}
