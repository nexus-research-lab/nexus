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

	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/argx"
)

// SessionTarget 按 execution_mode 推导出底层 SessionTarget。
func SessionTarget(args map[string]any, sctx contract.ServerContext, executionMode string) (automationsvc.SessionTarget, error) {
	switch executionMode {
	case "":
		return automationsvc.SessionTarget{}, errors.New("execution_mode is required (main / existing / temporary / dedicated)")
	case "main":
		if !sctx.IsMainAgent {
			return automationsvc.SessionTarget{}, errors.New("execution_mode=main is reserved for the main agent; regular agents must use existing / temporary / dedicated")
		}
		return automationsvc.SessionTarget{Kind: automationsvc.SessionTargetMain, WakeMode: automationsvc.WakeModeNextHeartbeat}.Normalized(), nil
	case "existing":
		bound := argx.FirstNonEmpty(argx.String(args, "selected_session_key"), sctx.CurrentSessionKey)
		if bound == "" {
			return automationsvc.SessionTarget{}, errors.New("execution_mode=existing requires selected_session_key (or an active current session). Pick a session via AskUserQuestion if unsure")
		}
		target := automationsvc.SessionTarget{Kind: automationsvc.SessionTargetBound, BoundSessionKey: bound}.Normalized()
		if err := target.Validate(); err != nil {
			return automationsvc.SessionTarget{}, err
		}
		return target, nil
	case "temporary":
		return automationsvc.SessionTarget{Kind: automationsvc.SessionTargetIsolated, WakeMode: automationsvc.WakeModeNextHeartbeat}.Normalized(), nil
	case "dedicated":
		name := argx.String(args, "named_session_key")
		if name == "" {
			return automationsvc.SessionTarget{}, errors.New("execution_mode=dedicated requires named_session_key. Use AskUserQuestion to confirm a dedicated session name first")
		}
		target := automationsvc.SessionTarget{Kind: automationsvc.SessionTargetNamed, NamedSessionKey: name}.Normalized()
		if err := target.Validate(); err != nil {
			return automationsvc.SessionTarget{}, err
		}
		return target, nil
	default:
		return automationsvc.SessionTarget{}, fmt.Errorf("unsupported execution_mode: %s (allowed: main / existing / temporary / dedicated)", executionMode)
	}
}

// Delivery 按 reply_mode 推导出底层 DeliveryTarget。
func Delivery(args map[string]any, sctx contract.ServerContext, executionMode, replyMode string, sessionTarget automationsvc.SessionTarget) (automationsvc.DeliveryTarget, error) {
	switch replyMode {
	case "":
		return automationsvc.DeliveryTarget{}, errors.New("reply_mode is required (none / execution / selected)")
	case "none":
		return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeNone}.Normalized(), nil
	case "execution":
		return executionReply(executionMode, args, sctx, sessionTarget)
	case "selected":
		to := argx.String(args, "selected_reply_session_key")
		if to == "" {
			return automationsvc.DeliveryTarget{}, errors.New("reply_mode=selected requires selected_reply_session_key. Use AskUserQuestion to confirm which existing session should receive the result")
		}
		return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeExplicit, Channel: "websocket", To: to}.Normalized(), nil
	default:
		return automationsvc.DeliveryTarget{}, fmt.Errorf("unsupported reply_mode: %s (allowed: none / execution / selected)", replyMode)
	}
}

// executionReply 处理 reply_mode=execution 的复杂分支：
// 主会话/agent 上下文下的 temporary、dedicated 默认不投递，避免重复轰炸。
func executionReply(executionMode string, args map[string]any, sctx contract.ServerContext, sessionTarget automationsvc.SessionTarget) (automationsvc.DeliveryTarget, error) {
	if sessionTarget.Kind == automationsvc.SessionTargetMain {
		return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeNone}.Normalized(), nil
	}
	resolved := executionMode
	if resolved == "" {
		resolved = executionModeFromTarget(sessionTarget)
	}
	if (resolved == "temporary" || resolved == "dedicated") && sctx.SourceContextType != "room" {
		return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeNone}.Normalized(), nil
	}
	to := argx.FirstNonEmpty(argx.String(args, "selected_session_key"), sctx.CurrentSessionKey)
	if to == "" {
		return automationsvc.DeliveryTarget{}, errors.New("reply_mode=execution requires selected_session_key or an active current session. Use AskUserQuestion to confirm which execution session should receive the result")
	}
	return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeExplicit, Channel: "websocket", To: to}.Normalized(), nil
}

func executionModeFromTarget(target automationsvc.SessionTarget) string {
	switch target.Kind {
	case automationsvc.SessionTargetBound:
		return "existing"
	case automationsvc.SessionTargetIsolated:
		return "temporary"
	case automationsvc.SessionTargetNamed:
		return "dedicated"
	case automationsvc.SessionTargetMain:
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
func Source(sctx contract.ServerContext, agentID string) automationsvc.Source {
	contextLabel := sctx.CurrentAgentName
	if contextLabel == "" {
		contextLabel = agentID
	}
	source := automationsvc.Source{
		Kind:           automationsvc.SourceKindAgent,
		CreatorAgentID: sctx.CurrentAgentID,
		ContextType:    "agent",
		ContextID:      agentID,
		ContextLabel:   contextLabel,
		SessionKey:     sctx.CurrentSessionKey,
		SessionLabel:   argx.FirstNonEmpty(sctx.CurrentSessionLabel, sessionLabelFallback(sctx.CurrentSessionKey)),
	}
	return source.Normalized()
}

func sessionLabelFallback(sessionKey string) string {
	if sessionKey == "" {
		return ""
	}
	return "当前对话"
}
