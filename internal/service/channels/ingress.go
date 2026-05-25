package channels

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
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

var defaultScheduledTaskApprovedTools = map[string]struct{}{
	"create_scheduled_task":           {},
	"delete_scheduled_task":           {},
	"disable_scheduled_task":          {},
	"enable_scheduled_task":           {},
	"get_scheduled_task_daily_report": {},
	"get_scheduled_task_events":       {},
	"get_scheduled_task_runs":         {},
	"get_scheduled_task_status":       {},
	"list_scheduled_tasks":            {},
	"recover_scheduled_task":          {},
	"retry_scheduled_task_delivery":   {},
	"run_scheduled_task":              {},
	"search_scheduled_task_history":   {},
	"update_scheduled_task":           {},
}

var defaultScheduledTaskSupportTools = map[string]struct{}{
	"Skill": {},
}

var defaultExternalApprovedTools = toolpolicy.MergeSets(defaultReadOnlyApprovedTools, defaultScheduledTaskApprovedTools)

// DMHandler 定义统一 DM 入口能力。
type DMHandler interface {
	HandleChat(context.Context, dmsvc.Request) error
}

// IngressRequest 表示一条来自外部通道的标准化消息。
type IngressRequest struct {
	Channel          string          `json:"channel,omitempty"`
	OwnerUserID      string          `json:"owner_user_id,omitempty"`
	SessionKey       string          `json:"session_key,omitempty"`
	AgentID          string          `json:"agent_id,omitempty"`
	ChatType         string          `json:"chat_type,omitempty"`
	Ref              string          `json:"ref,omitempty"`
	ThreadID         string          `json:"thread_id,omitempty"`
	ExternalName     string          `json:"external_name,omitempty"`
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
	Duplicate          bool            `json:"duplicate,omitempty"`
	RememberedDelivery *DeliveryTarget `json:"remembered_delivery,omitempty"`
}

type normalizedIngressRequest struct {
	ownerUserID      string
	channelStored    string
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
}

// IngressService 负责把外部通道消息归一到 DM 入口。
type IngressService struct {
	config    config.Config
	agents    agentWorkspaceResolver
	dm        DMHandler
	router    *Router
	control   *ControlService
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

// SetControlService 注入频道配置与配对授权服务。
func (s *IngressService) SetControlService(control *ControlService) {
	s.control = control
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

	claimedIngress := false
	if s.control != nil && normalized.reqID != "" {
		claimed, duplicate, claimErr := s.control.claimIngressMessage(ctx, ingressMessageClaimInput{
			OwnerUserID: normalized.ownerUserID,
			Channel:     normalized.channelStored,
			ReqID:       normalized.reqID,
			AgentID:     normalized.agentID,
			SessionKey:  normalized.sessionKey,
			RoundID:     normalized.roundID,
		})
		if claimErr != nil {
			logger.Error("领取通道消息幂等处理权失败", "err", claimErr)
			return nil, claimErr
		}
		if !claimed {
			logger.Info("忽略重复外部通道消息")
			return duplicate, nil
		}
		claimedIngress = true
	}

	agentValue, err := s.agents.GetAgent(ctx, normalized.agentID)
	if err != nil {
		logger.Error("解析通道消息目标 Agent 失败", "err", err)
		s.markIngressMessageFailed(ctx, claimedIngress, normalized, err)
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
		s.markIngressMessageFailed(ctx, claimedIngress, normalized, err)
		return nil, err
	}
	if claimedIngress {
		if err = s.control.finishIngressMessage(ctx, ingressMessageFinishInput{
			OwnerUserID: normalized.ownerUserID,
			Channel:     normalized.channelStored,
			ReqID:       normalized.reqID,
			Status:      ingressMessageStatusAccepted,
		}); err != nil {
			logger.Error("标记通道消息幂等状态失败", "err", err)
			return nil, err
		}
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

func (s *IngressService) markIngressMessageFailed(ctx context.Context, claimed bool, request normalizedIngressRequest, err error) {
	if !claimed || s.control == nil || err == nil {
		return
	}
	message := err.Error()
	if finishErr := s.control.finishIngressMessage(ctx, ingressMessageFinishInput{
		OwnerUserID:  request.ownerUserID,
		Channel:      request.channelStored,
		ReqID:        request.reqID,
		Status:       ingressMessageStatusFailed,
		ErrorMessage: &message,
	}); finishErr != nil {
		s.loggerFor(ctx).Warn("标记通道消息失败幂等状态失败",
			"channel", request.channelStored,
			"req_id", request.reqID,
			"err", finishErr,
		)
	}
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
		ownerUserID:      normalizeChannelOwnerUserID(firstNonEmptyIngress(request.OwnerUserID, authctx.OwnerUserID(ctx))),
		channelStored:    channelStored,
		sessionKey:       sessionKey,
		parsed:           parsed,
		agentID:          agentID,
		content:          content,
		roundID:          roundID,
		reqID:            firstNonEmptyIngress(request.ReqID, request.RoundID, roundID),
		permissionMode:   sdkpermission.Mode(strings.TrimSpace(request.PermissionMode)),
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
	if agentID == "" && s.control != nil {
		resolvedAgentID, pairErr := s.control.ResolveIngressAgent(ctx, request)
		if pairErr != nil {
			return "", protocol.SessionKey{}, "", pairErr
		}
		agentID = strings.TrimSpace(resolvedAgentID)
	}
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
	case ChannelTypeTelegram, ChannelTypeDingTalk, ChannelTypeWeChat, ChannelTypeFeishu:
		return deliveryTargetFromSessionRef(channelStored, parsed), nil
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

func deliveryTargetFromSessionRef(channel string, parsed protocol.SessionKey) *DeliveryTarget {
	ref := strings.TrimSpace(parsed.Ref)
	if ref == "" {
		return nil
	}
	return &DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  channel,
		To:       ref,
		ThreadID: strings.TrimSpace(parsed.ThreadID),
	}
}

func (s *IngressService) resolveApprovedTools(channel string, explicit []string) map[string]struct{} {
	if len(explicit) > 0 {
		return toolpolicy.NormalizeSet(explicit)
	}
	if channel == ChannelTypeInternal {
		return nil
	}
	return toolpolicy.CopySet(defaultExternalApprovedTools)
}

func (s *IngressService) buildPermissionHandler(
	agentValue *protocol.Agent,
	request normalizedIngressRequest,
) sdkpermission.Handler {
	allowedByAgent := toolpolicy.NormalizeSet(agentValue.Options.AllowedTools)
	approved := request.autoApproveTools
	if request.channelStored == ChannelTypeInternal && len(approved) == 0 {
		approved = toolpolicy.CopySet(allowedByAgent)
	}
	return func(_ context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
		toolName := strings.TrimSpace(permissionRequest.ToolName)
		if toolName == "" {
			return sdkpermission.Deny("permission tool_name is required", true), nil
		}
		// 外部通道没有前端问答能力，AskUserQuestion 必须直接拒绝，
		// 否则 SDK 会卡在等待人工输入，导致整个会话超时。
		if toolName == "AskUserQuestion" {
			return sdkpermission.Deny("当前通道不支持交互式问题确认", true), nil
		}
		if request.autoApproveAll {
			return sdkpermission.Allow(permissionRequest.Input, nil), nil
		}
		if len(allowedByAgent) > 0 {
			if !toolpolicy.Contains(allowedByAgent, toolName) && !isManagedScheduledTaskIngressTool(toolName) {
				return sdkpermission.Deny("当前 agent 未授权该工具", false), nil
			}
		}
		if len(approved) == 0 {
			return sdkpermission.Deny("当前通道未配置自动授权工具", false), nil
		}
		if !toolpolicy.Contains(approved, toolName) {
			return sdkpermission.Deny("当前通道不允许自动授权该工具", false), nil
		}
		return sdkpermission.Allow(permissionRequest.Input, nil), nil
	}
}

func isManagedScheduledTaskTool(toolName string) bool {
	return toolpolicy.Contains(defaultScheduledTaskApprovedTools, toolName)
}

func isManagedScheduledTaskIngressTool(toolName string) bool {
	return isManagedScheduledTaskTool(toolName) || toolpolicy.Contains(defaultScheduledTaskSupportTools, toolName)
}

func splitDiscordRoute(ref string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(ref), ":", 2)
	if len(parts) == 1 {
		return "", strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func firstNonEmptyIngress(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
