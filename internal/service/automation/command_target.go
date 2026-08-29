// INPUT: runtime command Actor 与模型表达的上下文/投递意图。
// OUTPUT: 宿主绑定的目标 Agent、SessionTarget、DeliveryTarget 和不可伪造 Source。
// POS: Automation command 跨 Agent、Room、IM 与 Session 路由的唯一翻译边界。
package automation

import (
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func runtimeCommandAgentID(actor command.Actor, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	current := strings.TrimSpace(actor.AgentID)
	if requested == "" || requested == current {
		return current, nil
	}
	if !actor.CrossAgentAllowed() {
		return "", fmt.Errorf("当前 runtime 只能管理 Agent %s 的自动化", current)
	}
	return requested, nil
}

func runtimeCommandSource(actor command.Actor) automationdomain.Source {
	contextType := "agent"
	if strings.TrimSpace(actor.SourceContextType) == "room" {
		contextType = "room"
	}
	contextID := strings.TrimSpace(actor.SourceContextID)
	if contextID == "" {
		contextID = strings.TrimSpace(actor.AgentID)
	}
	contextLabel := strings.TrimSpace(actor.SourceContextLabel)
	if contextLabel == "" {
		contextLabel = strings.TrimSpace(actor.AgentName)
	}
	return automationdomain.Source{
		Kind:           automationdomain.SourceKindAgent,
		CreatorAgentID: strings.TrimSpace(actor.AgentID),
		ContextType:    contextType,
		ContextID:      contextID,
		ContextLabel:   contextLabel,
		SessionKey:     strings.TrimSpace(actor.SessionKey),
		SessionLabel:   strings.TrimSpace(actor.SessionLabel),
	}.Normalized()
}

func runtimeCommandTargets(
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
) (automationdomain.SessionTarget, automationdomain.DeliveryTarget, error) {
	if strings.TrimSpace(input.DeliveryChannel) != "" {
		return automationdomain.SessionTarget{}, automationdomain.DeliveryTarget{},
			errors.New("delivery_channel 只用于查询 delivery_targets；创建或修改任务请使用宿主返回的 delivery_session_key")
	}
	advanced := strings.TrimSpace(input.ExecutionMode) != "" ||
		strings.TrimSpace(input.ReplyMode) != "" ||
		strings.TrimSpace(input.SelectedSessionKey) != "" ||
		strings.TrimSpace(input.NamedSessionKey) != "" ||
		strings.TrimSpace(input.SelectedReplySessionKey) != "" ||
		strings.TrimSpace(input.ReplySessionKey) != ""
	if advanced && !actor.CrossAgentAllowed() {
		return automationdomain.SessionTarget{}, automationdomain.DeliveryTarget{},
			errors.New("execution/reply Session 只能由主智能体自己的可信 Nexus 私有 DM 选择")
	}
	executionMode := strings.TrimSpace(input.ExecutionMode)
	if executionMode == "" {
		switch strings.TrimSpace(input.ContextMode) {
		case "current":
			executionMode = "existing"
		case "", "isolated":
			executionMode = "temporary"
		default:
			return automationdomain.SessionTarget{}, automationdomain.DeliveryTarget{},
				fmt.Errorf("context_mode 必须是 current 或 isolated")
		}
	}
	target, err := runtimeCommandSessionTarget(actor, input, executionMode)
	if err != nil {
		return automationdomain.SessionTarget{}, automationdomain.DeliveryTarget{}, err
	}
	replyMode := strings.TrimSpace(input.ReplyMode)
	if strings.TrimSpace(input.DeliverySessionKey) != "" {
		if replyMode != "" {
			return automationdomain.SessionTarget{}, automationdomain.DeliveryTarget{},
				errors.New("delivery_session_key 与 reply_mode 不能同时使用")
		}
		if input.DeliverResult != nil && !*input.DeliverResult {
			return automationdomain.SessionTarget{}, automationdomain.DeliveryTarget{},
				errors.New("delivery_session_key 与 deliver_result=false 不能同时使用")
		}
		replyMode = "selected"
	}
	if replyMode == "" {
		deliver := strings.TrimSpace(actor.SessionKey) != ""
		if input.DeliverResult != nil {
			deliver = *input.DeliverResult
		}
		if !deliver {
			replyMode = "none"
		} else if runtimeCommandExternalSession(actor.SessionKey) {
			replyMode = "channel"
		} else if executionMode == "existing" {
			replyMode = "execution"
		} else {
			replyMode = "selected"
		}
	}
	delivery, err := runtimeCommandDelivery(actor, input, executionMode, replyMode, target)
	if err != nil {
		return automationdomain.SessionTarget{}, automationdomain.DeliveryTarget{}, err
	}
	return target, delivery, nil
}

func runtimeCommandSessionTarget(
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
	mode string,
) (automationdomain.SessionTarget, error) {
	switch mode {
	case "main":
		if !actor.CrossAgentAllowed() {
			return automationdomain.SessionTarget{}, errors.New("execution_mode=main 只允许主智能体私有 DM")
		}
		return automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetMain, WakeMode: automationdomain.WakeModeNextHeartbeat,
		}.Normalized(), nil
	case "existing":
		sessionKey := strings.TrimSpace(input.SelectedSessionKey)
		if sessionKey == "" {
			sessionKey = strings.TrimSpace(actor.SessionKey)
		}
		if sessionKey == "" {
			return automationdomain.SessionTarget{}, errors.New("existing execution requires a trusted current or selected Session")
		}
		target := automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetBound, BoundSessionKey: sessionKey,
		}.Normalized()
		return target, target.Validate()
	case "temporary":
		return automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated, WakeMode: automationdomain.WakeModeNextHeartbeat,
		}.Normalized(), nil
	case "dedicated":
		if !actor.CrossAgentAllowed() {
			return automationdomain.SessionTarget{}, errors.New("dedicated execution 只允许主智能体私有 DM")
		}
		name := strings.TrimSpace(input.NamedSessionKey)
		if name == "" {
			return automationdomain.SessionTarget{}, errors.New("execution_mode=dedicated requires named_session_key")
		}
		target := automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetNamed, NamedSessionKey: name,
		}.Normalized()
		return target, target.Validate()
	default:
		return automationdomain.SessionTarget{}, fmt.Errorf("execution_mode 必须是 main、existing、temporary 或 dedicated")
	}
}

func runtimeCommandDelivery(
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
	executionMode string,
	replyMode string,
	target automationdomain.SessionTarget,
) (automationdomain.DeliveryTarget, error) {
	switch replyMode {
	case "none":
		return automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}.Normalized(), nil
	case "execution":
		if target.Kind == automationdomain.SessionTargetMain {
			return automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}.Normalized(), nil
		}
		if (executionMode == "temporary" || executionMode == "dedicated") &&
			strings.TrimSpace(actor.SourceContextType) != "room" {
			return automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}.Normalized(), nil
		}
		sessionKey := strings.TrimSpace(input.SelectedSessionKey)
		if sessionKey == "" {
			sessionKey = strings.TrimSpace(actor.SessionKey)
		}
		if sessionKey == "" {
			return automationdomain.DeliveryTarget{}, errors.New("reply_mode=execution 缺少可信 Session")
		}
		return runtimeCommandDeliveryFromSession(sessionKey), nil
	case "selected":
		sessionKey := strings.TrimSpace(input.DeliverySessionKey)
		if sessionKey == "" {
			sessionKey = strings.TrimSpace(input.SelectedReplySessionKey)
		}
		if sessionKey == "" {
			sessionKey = strings.TrimSpace(actor.SessionKey)
		}
		if sessionKey == "" {
			return automationdomain.DeliveryTarget{}, errors.New("selected delivery requires delivery_session_key or selected_reply_session_key")
		}
		if _, err := protocol.RequireStructuredSessionKey(sessionKey); err != nil {
			return automationdomain.DeliveryTarget{}, err
		}
		return runtimeCommandDeliveryFromSession(sessionKey), nil
	case "channel":
		if runtimeCommandExternalSession(actor.SessionKey) {
			return runtimeCommandDeliveryFromSession(actor.SessionKey), nil
		}
		sessionKey := strings.TrimSpace(input.ReplySessionKey)
		if sessionKey == "" {
			sessionKey = strings.TrimSpace(input.SelectedReplySessionKey)
		}
		if sessionKey == "" {
			return automationdomain.DeliveryTarget{}, errors.New("reply_mode=channel requires an existing authorized reply_session_key")
		}
		return runtimeCommandDeliveryFromSession(sessionKey), nil
	default:
		return automationdomain.DeliveryTarget{}, fmt.Errorf("reply_mode 必须是 none、execution、selected 或 channel")
	}
}

func runtimeCommandDeliveryFromSession(sessionKey string) automationdomain.DeliveryTarget {
	normalized := strings.TrimSpace(sessionKey)
	parsed := protocol.ParseSessionKey(normalized)
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	if parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent {
		switch channel {
		case protocol.SessionChannelWebSocket,
			protocol.SessionChannelInternalSegment:
			return automationdomain.DeliveryTarget{
				Mode: automationdomain.DeliveryModeExplicit, Channel: channel, To: normalized,
			}.Normalized()
		case protocol.SessionChannelDiscord,
			protocol.SessionChannelTelegram,
			protocol.SessionChannelDingTalk,
			protocol.SessionChannelWeChat,
			protocol.SessionChannelWeixinPersonal,
			protocol.SessionChannelFeishu:
			return automationdomain.DeliveryTarget{
				Mode: automationdomain.DeliveryModeLast, SessionKey: normalized,
			}.Normalized()
		}
	}
	return automationdomain.DeliveryTarget{
		Mode: automationdomain.DeliveryModeExplicit, Channel: "websocket", To: normalized,
	}.Normalized()
}

func runtimeCommandExternalSession(sessionKey string) bool {
	parsed := protocol.ParseSessionKey(strings.TrimSpace(sessionKey))
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent || strings.TrimSpace(parsed.Ref) == "" {
		return false
	}
	switch protocol.NormalizeStoredChannelType(parsed.Channel) {
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
		return true
	default:
		return false
	}
}
