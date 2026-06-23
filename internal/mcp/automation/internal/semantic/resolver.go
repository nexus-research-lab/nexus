// Package semantic 承载页面语义(execution_mode/reply_mode) 到 automation 底层
// SessionTarget / DeliveryTarget / Source 的翻译、校验与默认值守卫。
//
// 工具层只接受页面语义字段（execution_mode / reply_mode 等），
// 不再允许直接塞底层 session_target / delivery / source 对象——
// 这样 Agent 看到的入参永远和 UI「新建任务」对话框一一对应。
package semantic

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SessionTarget 按 execution_mode 推导出底层 SessionTarget。
func SessionTarget(args map[string]any, sctx contract.ServerContext, executionMode string) (protocol.SessionTarget, error) {
	switch executionMode {
	case "":
		return protocol.SessionTarget{}, errors.New("execution_mode is required (main / existing / temporary / dedicated)")
	case "main":
		if !sctx.IsMainAgent {
			return protocol.SessionTarget{}, errors.New("execution_mode=main is reserved for the main agent; regular agents must use existing / temporary / dedicated")
		}
		return protocol.SessionTarget{Kind: protocol.SessionTargetMain, WakeMode: protocol.WakeModeNextHeartbeat}.Normalized(), nil
	case "existing":
		bound := argx.FirstNonEmpty(argx.String(args, "selected_session_key"), sctx.CurrentSessionKey)
		if bound == "" {
			return protocol.SessionTarget{}, errors.New("execution_mode=existing requires selected_session_key (or an active current session). Ask the user in the current conversation to confirm the target session, then retry")
		}
		target := protocol.SessionTarget{Kind: protocol.SessionTargetBound, BoundSessionKey: bound}.Normalized()
		if err := target.Validate(); err != nil {
			return protocol.SessionTarget{}, err
		}
		return target, nil
	case "temporary":
		return protocol.SessionTarget{Kind: protocol.SessionTargetIsolated, WakeMode: protocol.WakeModeNextHeartbeat}.Normalized(), nil
	case "dedicated":
		name := argx.String(args, "named_session_key")
		if name == "" {
			return protocol.SessionTarget{}, errors.New("execution_mode=dedicated requires named_session_key. Ask the user in the current conversation to confirm a dedicated session name first")
		}
		target := protocol.SessionTarget{Kind: protocol.SessionTargetNamed, NamedSessionKey: name}.Normalized()
		if err := target.Validate(); err != nil {
			return protocol.SessionTarget{}, err
		}
		return target, nil
	default:
		return protocol.SessionTarget{}, fmt.Errorf("unsupported execution_mode: %s (allowed: main / existing / temporary / dedicated)", executionMode)
	}
}

// Delivery 按 reply_mode 推导出底层 DeliveryTarget。
func Delivery(args map[string]any, sctx contract.ServerContext, targetAgentID, executionMode, replyMode string, sessionTarget protocol.SessionTarget) (protocol.DeliveryTarget, error) {
	switch replyMode {
	case "":
		return protocol.DeliveryTarget{}, errors.New("reply_mode is required (none / execution / selected / agent / channel)")
	case "none":
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone}.Normalized(), nil
	case "execution":
		return executionReply(executionMode, args, sctx, sessionTarget)
	case "selected":
		to := argx.String(args, "selected_reply_session_key")
		if to == "" {
			return protocol.DeliveryTarget{}, errors.New("reply_mode=selected requires selected_reply_session_key. Ask the user in the current conversation to confirm which existing session should receive the result")
		}
		return deliveryFromSessionKey(to), nil
	case "agent":
		return agentReply(args, sctx, targetAgentID)
	case "channel":
		return channelReply(args, sctx)
	default:
		return protocol.DeliveryTarget{}, fmt.Errorf("unsupported reply_mode: %s (allowed: none / execution / selected / agent / channel)", replyMode)
	}
}

func agentReply(args map[string]any, sctx contract.ServerContext, targetAgentID string) (protocol.DeliveryTarget, error) {
	agentID := argx.FirstNonEmpty(argx.String(args, "reply_agent_id"), targetAgentID, sctx.CurrentAgentID)
	if strings.TrimSpace(agentID) == "" {
		return protocol.DeliveryTarget{}, errors.New("reply_mode=agent requires reply_agent_id or an active target agent")
	}
	sessionKey := protocol.BuildAgentSessionKey(
		strings.TrimSpace(agentID),
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	return protocol.DeliveryTarget{
		Mode:    protocol.DeliveryModeExplicit,
		Channel: protocol.SessionChannelInternalSegment,
		To:      sessionKey,
	}.Normalized(), nil
}

func channelReply(args map[string]any, sctx contract.ServerContext) (protocol.DeliveryTarget, error) {
	sessionKey := argx.FirstNonEmpty(argx.String(args, "reply_session_key"), argx.String(args, "selected_reply_session_key"))
	if sessionKey != "" {
		return deliveryFromSessionKey(sessionKey), nil
	}
	if !hasExplicitChannelReplyTarget(args) && currentSessionKeyCanDeliverToExternalChannel(sctx.CurrentSessionKey) {
		return deliveryFromSessionKey(sctx.CurrentSessionKey), nil
	}
	delivery := protocol.DeliveryTarget{
		Mode:      protocol.DeliveryModeExplicit,
		Channel:   argx.FirstNonEmpty(argx.String(args, "reply_channel"), argx.String(args, "delivery_channel")),
		To:        argx.FirstNonEmpty(argx.String(args, "reply_to"), argx.String(args, "delivery_to")),
		AccountID: argx.FirstNonEmpty(argx.String(args, "reply_account_id"), argx.String(args, "delivery_account_id")),
		ThreadID:  argx.FirstNonEmpty(argx.String(args, "reply_thread_id"), argx.String(args, "delivery_thread_id")),
	}
	if strings.TrimSpace(delivery.To) == "" {
		if filled, ok := fillChannelReplyTargetFromCurrentSession(delivery, sctx.CurrentSessionKey); ok {
			return filled.Normalized(), nil
		}
	}
	if strings.TrimSpace(delivery.Channel) == "" {
		return protocol.DeliveryTarget{}, errors.New("reply_mode=channel requires reply_channel or reply_session_key")
	}
	if strings.TrimSpace(delivery.To) == "" {
		return protocol.DeliveryTarget{}, errors.New("reply_mode=channel requires reply_to or reply_session_key")
	}
	return delivery.Normalized(), nil
}

func hasExplicitChannelReplyTarget(args map[string]any) bool {
	for _, key := range []string{
		"reply_channel", "delivery_channel",
		"reply_to", "delivery_to",
		"reply_account_id", "delivery_account_id",
		"reply_thread_id", "delivery_thread_id",
	} {
		if strings.TrimSpace(argx.String(args, key)) != "" {
			return true
		}
	}
	return false
}

func fillChannelReplyTargetFromCurrentSession(
	delivery protocol.DeliveryTarget,
	currentSessionKey string,
) (protocol.DeliveryTarget, bool) {
	if !currentSessionKeyCanDeliverToExternalChannel(currentSessionKey) {
		return protocol.DeliveryTarget{}, false
	}
	current := deliveryFromSessionKey(currentSessionKey)
	currentChannel := protocol.NormalizeStoredChannelType(current.Channel)
	requestedChannel := protocol.NormalizeStoredChannelType(delivery.Channel)
	if requestedChannel != "" && requestedChannel != currentChannel {
		return protocol.DeliveryTarget{}, false
	}
	result := delivery
	result.Mode = protocol.DeliveryModeExplicit
	if strings.TrimSpace(result.Channel) == "" {
		result.Channel = currentChannel
	}
	if strings.TrimSpace(result.To) == "" {
		result.To = strings.TrimSpace(current.To)
	}
	if strings.TrimSpace(result.ThreadID) == "" {
		result.ThreadID = strings.TrimSpace(current.ThreadID)
	}
	if strings.TrimSpace(result.AccountID) == "" {
		result.AccountID = strings.TrimSpace(current.AccountID)
	}
	return result, true
}

func currentSessionKeyCanDeliverToExternalChannel(sessionKey string) bool {
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return false
	}
	switch protocol.NormalizeStoredChannelType(parsed.Channel) {
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
		return strings.TrimSpace(parsed.Ref) != ""
	default:
		return false
	}
}

func deliveryFromSessionKey(sessionKey string) protocol.DeliveryTarget {
	normalized := strings.TrimSpace(sessionKey)
	parsed := protocol.ParseSessionKey(normalized)
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent || channel == "" {
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeExplicit, Channel: "websocket", To: normalized}.Normalized()
	}
	if channel == protocol.SessionChannelWebSocket || channel == protocol.SessionChannelInternalSegment {
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeExplicit, Channel: channel, To: normalized}.Normalized()
	}
	switch channel {
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
	default:
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeExplicit, Channel: "websocket", To: normalized}.Normalized()
	}
	return protocol.DeliveryTarget{
		Mode:     protocol.DeliveryModeExplicit,
		Channel:  channel,
		To:       parsed.Ref,
		ThreadID: parsed.ThreadID,
	}.Normalized()
}

// executionReply 处理 reply_mode=execution 的复杂分支：
// 主会话/agent 上下文下的 temporary、dedicated 默认不投递，避免重复轰炸。
func executionReply(executionMode string, args map[string]any, sctx contract.ServerContext, sessionTarget protocol.SessionTarget) (protocol.DeliveryTarget, error) {
	if sessionTarget.Kind == protocol.SessionTargetMain {
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone}.Normalized(), nil
	}
	resolved := executionMode
	if resolved == "" {
		resolved = executionModeFromTarget(sessionTarget)
	}
	if (resolved == "temporary" || resolved == "dedicated") && strings.TrimSpace(sctx.SourceContextType) != "room" {
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone}.Normalized(), nil
	}
	to := argx.FirstNonEmpty(argx.String(args, "selected_session_key"), sctx.CurrentSessionKey)
	if to == "" {
		return protocol.DeliveryTarget{}, errors.New("reply_mode=execution requires selected_session_key or an active current session. Ask the user in the current conversation to confirm which execution session should receive the result")
	}
	return deliveryFromSessionKey(to), nil
}

func executionModeFromTarget(target protocol.SessionTarget) string {
	switch target.Kind {
	case protocol.SessionTargetBound:
		return "existing"
	case protocol.SessionTargetIsolated:
		return "temporary"
	case protocol.SessionTargetNamed:
		return "dedicated"
	case protocol.SessionTargetMain:
		return "main"
	}
	return ""
}

// ValidatePage 收口页面语义下不允许的字段组合。
func ValidatePage(executionMode, replyMode string) error {
	execMode := strings.TrimSpace(executionMode)
	rplMode := strings.TrimSpace(replyMode)
	if execMode == "main" && rplMode != "" && rplMode != "none" {
		return errors.New("execution_mode=main does not support reply_mode under page semantics. To run independently and send the result back here, use temporary + selected")
	}
	return nil
}

// Source 基于当前 ServerContext 组装 Source 元数据。
// 工具层不再接受外部传入的 source 对象，统一使用当前上下文，避免 Agent 伪造来源。
func Source(sctx contract.ServerContext, agentID string) protocol.Source {
	contextType := "agent"
	if strings.TrimSpace(sctx.SourceContextType) == "room" {
		contextType = "room"
	}
	contextID := strings.TrimSpace(sctx.SourceContextID)
	if contextID == "" {
		if contextType == "room" {
			contextID = roomContextIDFallback(sctx.CurrentSessionKey)
		}
		if contextID == "" {
			contextID = agentID
		}
	}
	contextLabel := strings.TrimSpace(sctx.SourceContextLabel)
	if contextLabel == "" && contextType == "agent" {
		contextLabel = strings.TrimSpace(sctx.CurrentAgentName)
		if contextLabel == "" {
			contextLabel = agentID
		}
	}
	sessionLabel := strings.TrimSpace(sctx.CurrentSessionLabel)
	if sessionLabel == "" && sctx.CurrentSessionKey != "" {
		sessionLabel = "当前对话"
	}
	source := protocol.Source{
		Kind:           protocol.SourceKindAgent,
		CreatorAgentID: sctx.CurrentAgentID,
		ContextType:    contextType,
		ContextID:      contextID,
		ContextLabel:   contextLabel,
		SessionKey:     sctx.CurrentSessionKey,
		SessionLabel:   sessionLabel,
	}
	return source.Normalized()
}

func roomContextIDFallback(sessionKey string) string {
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return parsed.ConversationID
	}
	if parsed.Kind == protocol.SessionKeyKindAgent && (parsed.ChatType == "group" || parsed.ChatType == "dm") {
		return parsed.Ref
	}
	return ""
}
