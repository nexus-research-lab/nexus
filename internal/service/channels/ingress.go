package channels

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

var (
	// ErrIngressChannelRequired 表示入口缺少 channel。
	ErrIngressChannelRequired = errors.New("channel is required")
	// ErrIngressRefRequired 表示结构化入口缺少 ref。
	ErrIngressRefRequired = errors.New("ref is required when session_key is empty")
)

var defaultReadOnlyApprovedTools = map[string]struct{}{
	"Glob":      {},
	"Grep":      {},
	"LS":        {},
	"Read":      {},
	"Skill":     {},
	"WebFetch":  {},
	"WebSearch": {},
}

// DMHandler 定义统一 DM 入口能力。
type DMHandler interface {
	HandleChat(context.Context, dmsvc.Request) error
}

// IngressRequest 表示一条来自外部通道的标准化消息。
type IngressRequest struct {
	Channel          string          `json:"channel,omitempty"`
	SessionKey       string          `json:"session_key,omitempty"`
	AgentID          string          `json:"agent_id,omitempty"`
	ChatType         string          `json:"chat_type,omitempty"`
	Ref              string          `json:"ref,omitempty"`
	ThreadID         string          `json:"thread_id,omitempty"`
	Content          string          `json:"content"`
	RoundID          string          `json:"round_id,omitempty"`
	ReqID            string          `json:"req_id,omitempty"`
	PermissionMode   string          `json:"permission_mode,omitempty"`
	AutoApproveAll   bool            `json:"auto_approve_all,omitempty"`
	AutoApproveTools []string        `json:"auto_approve_tools,omitempty"`
	Delivery         *DeliveryTarget `json:"delivery,omitempty"`
}

// IngressResult 描述入口受理结果。
type IngressResult struct {
	Channel            string          `json:"channel"`
	AgentID            string          `json:"agent_id"`
	SessionKey         string          `json:"session_key"`
	RoundID            string          `json:"round_id"`
	ReqID              string          `json:"req_id"`
	RememberedDelivery *DeliveryTarget `json:"remembered_delivery,omitempty"`
}

type normalizedIngressRequest struct {
	channelStored    string
	sessionKey       string
	parsed           protocol.SessionKey
	agentID          string
	content          string
	roundID          string
	reqID            string
	permissionMode   sdkprotocol.PermissionMode
	autoApproveAll   bool
	autoApproveTools map[string]struct{}
	rememberedTarget *DeliveryTarget
}

// IngressService 负责把外部通道消息归一到 DM 入口。
type IngressService struct {
	config    config.Config
	agents    agentWorkspaceResolver
	dm        DMHandler
	router    *Router
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
		idFactory: newDeliveryID,
		logger:    logx.NewDiscardLogger(),
	}
}

// SetLogger 注入业务日志实例。
func (s *IngressService) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// Accept 受理一条外部通道消息。
func (s *IngressService) Accept(ctx context.Context, request IngressRequest) (*IngressResult, error) {
	normalized, err := s.normalizeRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	if s.agents == nil {
		return nil, errors.New("ingress service is not configured with agent resolver")
	}
	if s.dm == nil {
		return nil, errors.New("ingress service is not configured with dm handler")
	}

	logger := s.loggerFor(ctx).With(
		"channel", normalized.channelStored,
		"agent_id", normalized.agentID,
		"session_key", normalized.sessionKey,
		"round_id", normalized.roundID,
		"req_id", normalized.reqID,
	)
	logger.Info("受理外部通道消息",
		"content_chars", utf8.RuneCountInString(normalized.content),
	)

	agentValue, err := s.agents.GetAgent(ctx, normalized.agentID)
	if err != nil {
		logger.Error("解析通道消息目标 Agent 失败", "err", err)
		return nil, err
	}
	if err = s.dm.HandleChat(ctx, dmsvc.Request{
		SessionKey:        normalized.sessionKey,
		AgentID:           normalized.agentID,
		Content:           normalized.content,
		RoundID:           normalized.roundID,
		ReqID:             normalized.reqID,
		PermissionMode:    normalized.permissionMode,
		PermissionHandler: s.buildPermissionHandler(agentValue, normalized),
	}); err != nil {
		logger.Error("下发通道消息失败", "err", err)
		return nil, err
	}

	var remembered *DeliveryTarget
	if normalized.rememberedTarget != nil && s.router != nil {
		remembered, err = s.router.RememberRoute(ctx, normalized.agentID, *normalized.rememberedTarget)
		if err != nil {
			logger.Error("记录通道回投目标失败", "err", err)
			return nil, err
		}
	}
	logger.Info("通道消息已进入 DM 主链",
		"remembered_delivery", remembered != nil,
	)

	return &IngressResult{
		Channel:            normalized.channelStored,
		AgentID:            normalized.agentID,
		SessionKey:         normalized.sessionKey,
		RoundID:            normalized.roundID,
		ReqID:              normalized.reqID,
		RememberedDelivery: remembered,
	}, nil
}

func (s *IngressService) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

func (s *IngressService) normalizeRequest(ctx context.Context, request IngressRequest) (normalizedIngressRequest, error) {
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return normalizedIngressRequest{}, errors.New("content is required")
	}

	sessionKey, parsed, agentID, err := s.resolveSession(ctx, request)
	if err != nil {
		return normalizedIngressRequest{}, err
	}

	channelStored := protocol.NormalizeStoredChannelType(parsed.Channel)
	rememberedTarget, err := s.resolveRememberedTarget(channelStored, parsed, request.Delivery)
	if err != nil {
		return normalizedIngressRequest{}, err
	}
	roundID := firstNonEmptyIngress(request.RoundID, s.idFactory("ingress_round"))

	return normalizedIngressRequest{
		channelStored:    channelStored,
		sessionKey:       sessionKey,
		parsed:           parsed,
		agentID:          agentID,
		content:          content,
		roundID:          roundID,
		reqID:            firstNonEmptyIngress(request.ReqID, request.RoundID, roundID),
		permissionMode:   sdkprotocol.PermissionMode(strings.TrimSpace(request.PermissionMode)),
		autoApproveAll:   request.AutoApproveAll,
		autoApproveTools: s.resolveApprovedTools(channelStored, request.AutoApproveTools),
		rememberedTarget: rememberedTarget,
	}, nil
}

func (s *IngressService) resolveSession(ctx context.Context, request IngressRequest) (string, protocol.SessionKey, string, error) {
	if strings.TrimSpace(request.SessionKey) != "" {
		sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
		if err != nil {
			return "", protocol.SessionKey{}, "", err
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind != protocol.SessionKeyKindAgent {
			return "", protocol.SessionKey{}, "", errors.New("channel ingress 仅支持 agent session_key")
		}
		if channel := protocol.NormalizeSessionKeyChannelSegment(request.Channel); channel != "" && channel != protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) {
			return "", protocol.SessionKey{}, "", errors.New("channel 与 session_key 不一致")
		}
		if agentID := strings.TrimSpace(request.AgentID); agentID != "" && agentID != parsed.AgentID {
			return "", protocol.SessionKey{}, "", errors.New("agent_id 与 session_key 不一致")
		}
		return sessionKey, parsed, parsed.AgentID, nil
	}

	channel := protocol.NormalizeSessionKeyChannelSegment(request.Channel)
	if channel == "" {
		return "", protocol.SessionKey{}, "", ErrIngressChannelRequired
	}
	ref := strings.TrimSpace(request.Ref)
	if ref == "" {
		return "", protocol.SessionKey{}, "", ErrIngressRefRequired
	}

	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" {
		if s.agents == nil {
			return "", protocol.SessionKey{}, "", errors.New("channel ingress 缺少默认 agent 解析器")
		}
		defaultAgent, err := s.agents.GetDefaultAgent(ctx)
		if err != nil {
			return "", protocol.SessionKey{}, "", err
		}
		agentID = defaultAgent.AgentID
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentID,
		channel,
		protocol.NormalizeSessionChatType(request.ChatType),
		ref,
		strings.TrimSpace(request.ThreadID),
	)
	parsed := protocol.ParseSessionKey(sessionKey)
	return sessionKey, parsed, agentID, nil
}

func (s *IngressService) resolveRememberedTarget(
	channelStored string,
	parsed protocol.SessionKey,
	explicit *DeliveryTarget,
) (*DeliveryTarget, error) {
	if explicit != nil {
		target := explicit.Normalized()
		target.Mode = DeliveryModeExplicit
		if target.Channel == "" {
			target.Channel = channelStored
		}
		if target.Channel == ChannelTypeInternal && target.SessionKey == "" {
			target.SessionKey = parsed.Raw
		}
		if err := target.Validate(); err != nil {
			return nil, err
		}
		return &target, nil
	}

	switch channelStored {
	case ChannelTypeInternal:
		target := DeliveryTarget{
			Mode:       DeliveryModeExplicit,
			Channel:    ChannelTypeInternal,
			To:         parsed.Raw,
			SessionKey: parsed.Raw,
		}
		return &target, nil
	case ChannelTypeTelegram:
		target := DeliveryTarget{
			Mode:     DeliveryModeExplicit,
			Channel:  ChannelTypeTelegram,
			To:       strings.TrimSpace(parsed.Ref),
			ThreadID: strings.TrimSpace(parsed.ThreadID),
		}
		return &target, nil
	case ChannelTypeDiscord:
		if parsed.ChatType != "group" {
			return nil, nil
		}
		guildID, channelID := splitDiscordRoute(strings.TrimSpace(parsed.Ref))
		if channelID == "" {
			return nil, nil
		}
		target := DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeDiscord,
			To:        channelID,
			AccountID: guildID,
			ThreadID:  strings.TrimSpace(parsed.ThreadID),
		}
		return &target, nil
	default:
		return nil, nil
	}
}

func (s *IngressService) resolveApprovedTools(channel string, explicit []string) map[string]struct{} {
	if len(explicit) > 0 {
		return normalizeToolSet(explicit)
	}
	if channel == ChannelTypeInternal {
		return nil
	}
	return copyToolSet(defaultReadOnlyApprovedTools)
}

func (s *IngressService) buildPermissionHandler(
	agentValue *protocol.Agent,
	request normalizedIngressRequest,
) agentclient.PermissionHandler {
	allowedByAgent := normalizeToolSet(agentValue.Options.AllowedTools)
	approved := request.autoApproveTools
	if request.channelStored == ChannelTypeInternal && len(approved) == 0 {
		approved = copyToolSet(allowedByAgent)
	}
	return func(_ context.Context, permissionRequest sdkprotocol.PermissionRequest) (sdkprotocol.PermissionDecision, error) {
		toolName := strings.TrimSpace(permissionRequest.ToolName)
		if toolName == "" {
			return sdkprotocol.DenyPermission("permission tool_name is required", true), nil
		}
		// 外部通道没有前端问答能力，AskUserQuestion 必须直接拒绝，
		// 否则 SDK 会卡在等待人工输入，导致整个会话超时。
		if toolName == "AskUserQuestion" {
			return sdkprotocol.DenyPermission("当前通道不支持交互式问题确认", true), nil
		}
		if request.autoApproveAll {
			return sdkprotocol.AllowPermission(permissionRequest.Input, nil), nil
		}
		if len(allowedByAgent) > 0 {
			if _, ok := allowedByAgent[toolName]; !ok {
				return sdkprotocol.DenyPermission("当前 agent 未授权该工具", false), nil
			}
		}
		if len(approved) == 0 {
			return sdkprotocol.DenyPermission("当前通道未配置自动授权工具", false), nil
		}
		if _, ok := approved[toolName]; !ok {
			return sdkprotocol.DenyPermission("当前通道不允许自动授权该工具", false), nil
		}
		return sdkprotocol.AllowPermission(permissionRequest.Input, nil), nil
	}
}

func splitDiscordRoute(ref string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(ref), ":", 2)
	if len(parts) == 1 {
		return "", strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func normalizeToolSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		result[value] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func copyToolSet(items map[string]struct{}) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(items))
	for key := range items {
		result[key] = struct{}{}
	}
	return result
}

func firstNonEmptyIngress(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
