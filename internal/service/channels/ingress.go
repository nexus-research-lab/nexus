package channels

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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

// ExternalSessionNotifierFunc 适配函数式外部 session 通知器。
type ExternalSessionNotifierFunc func(context.Context, string, string)

// NotifyExternalSessionUpdated 实现 ExternalSessionNotifier。
func (fn ExternalSessionNotifierFunc) NotifyExternalSessionUpdated(ctx context.Context, agentID string, sessionKey string) {
	fn(ctx, agentID, sessionKey)
}

type normalizedIngressRequest struct {
	ownerUserID      string
	channelStored    string
	accountID        string
	sessionKey       string
	parsed           protocol.SessionKey
	agentID          string
	content          string
	roundID          string
	reqID            string
	permissionMode   sdkpermission.Mode
	autoApproveAll   bool
	autoApproveTools map[string]struct{}
	rememberedTarget *DeliveryTarget
	message          *channelmessage.Inbound
}

func (r normalizedIngressRequest) messageID() string {
	if r.message == nil {
		return ""
	}
	return strings.TrimSpace(r.message.PlatformMessageID)
}

// IngressService 负责把外部通道消息归一到 DM 入口。
type IngressService struct {
	config    config.Config
	agents    agentWorkspaceResolver
	dm        DMHandler
	router    *Router
	control   *ControlService
	notifier  ExternalSessionNotifier
	idFactory func(string) string
	logger    *slog.Logger
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
