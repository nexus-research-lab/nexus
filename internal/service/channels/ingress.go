package channels

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

var (
	// ErrIngressChannelRequired 表示入口缺少 channel。
	ErrIngressChannelRequired = errors.New("channel is required")
	// ErrIngressRefRequired 表示结构化入口缺少 ref。
	ErrIngressRefRequired = errors.New("ref is required when session_key is empty")
)

// DMHandler 定义统一 DM 入口能力。
type DMHandler interface {
	HandleChat(context.Context, dmsvc.Request) error
}

// ExternalSessionNotifier 接收外部通道 session 元数据更新通知。
type ExternalSessionNotifier interface {
	NotifyExternalSessionUpdated(context.Context, string, string)
}

// IngressCommandRequest 是已通过 channel pairing 校验的控制命令上下文。
type IngressCommandRequest struct {
	OwnerUserID string
	AgentID     string
	SessionKey  string
	Content     string
}

// IngressCommandResult 表示命令是否已由控制面消费，以及应回投给当前 IM 的文本。
type IngressCommandResult struct {
	Handled bool
	Reply   string
}

// IngressCommandHandler 在消息进入 Agent runtime 前消费可信 IM 控制命令。
type IngressCommandHandler interface {
	HandleIngressCommand(context.Context, IngressCommandRequest) (IngressCommandResult, error)
}

// IngressPermissionCommandInspector 让命令编排层在执行无 ID 权限命令前，
// 只读统计其他持久审批域中属于当前可信 IM session 的 pending 请求。
type IngressPermissionCommandInspector interface {
	CountPendingIngressPermissionRequests(context.Context, IngressCommandRequest) (int, error)
}

// ExternalSessionNotifierFunc 适配函数式外部 session 通知器。
type ExternalSessionNotifierFunc func(context.Context, string, string)

// NotifyExternalSessionUpdated 实现 ExternalSessionNotifier。
func (fn ExternalSessionNotifierFunc) NotifyExternalSessionUpdated(ctx context.Context, agentID string, sessionKey string) {
	fn(ctx, agentID, sessionKey)
}

type normalizedIngressRequest struct {
	ownerUserID                string
	channelStored              string
	accountID                  string
	sessionKey                 string
	parsed                     protocol.SessionKey
	agentID                    string
	content                    string
	roundID                    string
	reqID                      string
	permissionMode             sdkpermission.Mode
	autoApproveAll             bool
	autoApproveTools           map[string]struct{}
	trustedExternalInteractive bool
	rememberedTarget           *DeliveryTarget
	message                    *channelmessage.Inbound
}

func (r normalizedIngressRequest) messageID() string {
	if r.message == nil {
		return ""
	}
	return strings.TrimSpace(r.message.PlatformMessageID)
}

// IngressService 负责把外部通道消息归一到 DM 入口。
type IngressService struct {
	config     config.Config
	agents     agentWorkspaceResolver
	dm         DMHandler
	router     *Router
	control    *ControlService
	notifier   ExternalSessionNotifier
	commands   IngressCommandHandler
	permission *permissionctx.Context
	idFactory  func(string) string
	logger     *slog.Logger
}

// NewIngressService 创建通道入口服务。
func NewIngressService(
	cfg config.Config,
	agents agentWorkspaceResolver,
	dm DMHandler,
	router *Router,
) *IngressService {
	return &IngressService{
		config:    cfg,
		agents:    agents,
		dm:        dm,
		router:    router,
		idFactory: channelcontract.NewID,
		logger:    logx.NewDiscardLogger(),
	}
}

// SetControlService 注入频道配置与配对授权服务。
func (s *IngressService) SetControlService(control *ControlService) {
	s.control = control
}

// SetExternalSessionNotifier 注入外部 session 更新通知器。
func (s *IngressService) SetExternalSessionNotifier(notifier ExternalSessionNotifier) {
	s.notifier = notifier
}

// SetCommandHandler 注入外部 IM 控制命令处理器。
func (s *IngressService) SetCommandHandler(handler IngressCommandHandler) {
	s.commands = handler
}

// SetRuntimePermissionContext 注入普通 Agent 工具的阻塞式人工审批真相源。
func (s *IngressService) SetRuntimePermissionContext(permission *permissionctx.Context) {
	s.permission = permission
}

// SetLogger 注入业务日志实例。
func (s *IngressService) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

func (s *IngressService) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}
