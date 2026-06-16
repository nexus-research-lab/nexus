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
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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

var defaultGoalApprovedTools = map[string]struct{}{
	"create_goal": {},
	"get_goal":    {},
	"update_goal": {},
}

var defaultManagedSupportTools = map[string]struct{}{
	"Skill": {},
}

var defaultExternalApprovedTools = toolpolicy.MergeSets(defaultReadOnlyApprovedTools, defaultScheduledTaskApprovedTools, defaultGoalApprovedTools)

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
		idFactory: newDeliveryID,
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
		"account_id", normalized.accountID,
		"agent_id", normalized.agentID,
		"session_key", normalized.sessionKey,
		"round_id", normalized.roundID,
		"req_id", normalized.reqID,
	)
	logger.Info("受理外部通道消息",
		"content_chars", utf8.RuneCountInString(normalized.content),
		"platform_message_id", normalized.messageID(),
	)

	claimedIngress := false
	if s.control != nil && normalized.reqID != "" {
		claimed, duplicate, claimErr := s.control.claimIngressMessage(ctx, ingressMessageClaimInput{
			OwnerUserID: normalized.ownerUserID,
			Channel:     normalized.channelStored,
			AccountID:   normalized.accountID,
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

	ownerCtx := contextWithIngressOwner(ctx, normalized.ownerUserID)
	agentValue, err := s.agents.GetAgent(ownerCtx, normalized.agentID)
	if err != nil {
		logger.Error("解析通道消息目标 Agent 失败", "err", err)
		s.markIngressMessageFailed(ctx, claimedIngress, normalized, err)
		return nil, err
	}
	if err = s.dm.HandleChat(ownerCtx, dmsvc.Request{
		SessionKey:           normalized.sessionKey,
		AgentID:              normalized.agentID,
		Content:              normalized.content,
		RoundID:              normalized.roundID,
		ReqID:                normalized.reqID,
		PermissionMode:       normalized.permissionMode,
		BroadcastUserMessage: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Metadata: channelmessage.RuntimeMetadata(normalized.message),
		},
		PermissionHandler:   s.buildPermissionHandler(agentValue, normalized),
		ExternalReplyTarget: dmExternalReplyTarget(normalized.rememberedTarget),
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
		if _, err = s.router.RememberSessionRoute(ctx, normalized.agentID, normalized.sessionKey, *normalized.rememberedTarget); err != nil {
			logger.Error("记录通道 session 回投目标失败", "err", err)
			return nil, err
		}
	}
	logger.Info("通道消息已进入 DM 主链",
		"remembered_delivery", remembered != nil,
	)
	s.notifyExternalSessionUpdated(ctx, normalized)

	return &IngressResult{
		Channel:            normalized.channelStored,
		AgentID:            normalized.agentID,
		SessionKey:         normalized.sessionKey,
		RoundID:            normalized.roundID,
		ReqID:              normalized.reqID,
		RememberedDelivery: remembered,
		Message:            normalized.message,
	}, nil
}

func (s *IngressService) notifyExternalSessionUpdated(ctx context.Context, request normalizedIngressRequest) {
	if s.notifier == nil || !shouldNotifyExternalSessionUpdate(request.channelStored) {
		return
	}
	s.notifier.NotifyExternalSessionUpdated(ctx, request.agentID, request.sessionKey)
}

func shouldNotifyExternalSessionUpdate(channel string) bool {
	normalized := normalizeChannelType(channel)
	return normalized != "" && normalized != ChannelTypeInternal && normalized != ChannelTypeWebSocket
}

func (s *IngressService) markIngressMessageFailed(ctx context.Context, claimed bool, request normalizedIngressRequest, err error) {
	if !claimed || s.control == nil || err == nil {
		return
	}
	message := err.Error()
	if finishErr := s.control.finishIngressMessage(ctx, ingressMessageFinishInput{
		OwnerUserID:  request.ownerUserID,
		Channel:      request.channelStored,
		AccountID:    request.accountID,
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

	ownerUserID := normalizeChannelOwnerUserID(firstNonEmptyIngress(request.OwnerUserID, authctx.OwnerUserID(ctx)))
	ownerCtx := contextWithIngressOwner(ctx, ownerUserID)
	sessionKey, parsed, agentID, err := s.resolveSession(ownerCtx, request)
	if err != nil {
		return normalizedIngressRequest{}, err
	}

	channelStored := protocol.NormalizeStoredChannelType(parsed.Channel)
	accountID := strings.TrimSpace(parsed.AccountID)
	rememberedTarget, err := s.resolveRememberedTarget(channelStored, parsed, request.Delivery)
	if err != nil {
		return normalizedIngressRequest{}, err
	}
	roundID := firstNonEmptyIngress(request.RoundID, s.idFactory("ingress_round"))
	reqID := firstNonEmptyIngress(request.ReqID, request.RoundID, roundID)
	message := migrateIngressMessage(request, channelStored, parsed, content, reqID)

	return normalizedIngressRequest{
		ownerUserID:      ownerUserID,
		channelStored:    channelStored,
		accountID:        accountID,
		sessionKey:       sessionKey,
		parsed:           parsed,
		agentID:          agentID,
		content:          content,
		roundID:          roundID,
		reqID:            reqID,
		permissionMode:   sdkpermission.Mode(strings.TrimSpace(request.PermissionMode)),
		autoApproveAll:   request.AutoApproveAll,
		autoApproveTools: s.resolveApprovedTools(channelStored, request.AutoApproveTools),
		rememberedTarget: rememberedTarget,
		message:          message,
	}, nil
}

func contextWithIngressOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok && strings.TrimSpace(currentUserID) == ownerUserID {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
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
		if accountID := strings.TrimSpace(request.AccountID); accountID != "" && parsed.AccountID != "" && accountID != parsed.AccountID {
			return "", protocol.SessionKey{}, "", errors.New("account_id 与 session_key 不一致")
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
	accountID := strings.TrimSpace(request.AccountID)
	sessionKey := protocol.BuildAgentAccountSessionKey(
		agentID,
		channel,
		protocol.NormalizeSessionChatType(request.ChatType),
		accountID,
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
		if target.Channel == ChannelTypeWeixinPersonal && target.AccountID == "" {
			target.AccountID = strings.TrimSpace(parsed.AccountID)
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
	case ChannelTypeTelegram, ChannelTypeDingTalk, ChannelTypeWeChat, ChannelTypeWeixinPersonal, ChannelTypeFeishu:
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
	target := &DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  channel,
		To:       ref,
		ThreadID: strings.TrimSpace(parsed.ThreadID),
	}
	if channel == ChannelTypeWeixinPersonal {
		target.AccountID = strings.TrimSpace(parsed.AccountID)
	}
	return target
}

func dmExternalReplyTarget(target *DeliveryTarget) *dmsvc.ExternalReplyTarget {
	if target == nil {
		return nil
	}
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone {
		return nil
	}
	switch normalized.Channel {
	case "", ChannelTypeWebSocket, ChannelTypeInternal:
		return nil
	}
	return &dmsvc.ExternalReplyTarget{
		Mode:       normalized.Mode,
		Channel:    normalized.Channel,
		To:         normalized.To,
		AccountID:  normalized.AccountID,
		ThreadID:   normalized.ThreadID,
		SessionKey: normalized.SessionKey,
	}
}

func (s *IngressService) resolveApprovedTools(channel string, explicit []string) map[string]struct{} {
	if len(explicit) > 0 {
		return toolpolicy.NormalizeSet(explicit)
	}
	if channel == ChannelTypeInternal {
		return toolpolicy.CopySet(defaultReadOnlyApprovedTools)
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
		if len(allowedByAgent) > 0 {
			approved = toolpolicy.CopySet(allowedByAgent)
		} else {
			approved = toolpolicy.CopySet(defaultReadOnlyApprovedTools)
		}
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
			if !toolpolicy.Contains(allowedByAgent, toolName) && !isManagedIngressTool(toolName) {
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

func isManagedGoalTool(toolName string) bool {
	return toolpolicy.IsManagedGoalTool(toolName)
}

func isManagedIngressTool(toolName string) bool {
	return isManagedScheduledTaskTool(toolName) || isManagedGoalTool(toolName) || toolpolicy.Contains(defaultManagedSupportTools, toolName)
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
