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

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
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
			return protocol.SessionTarget{}, errors.New("execution_mode=existing requires selected_session_key (or an active current session). Pick a session via AskUserQuestion if unsure")
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
			return protocol.SessionTarget{}, errors.New("execution_mode=dedicated requires named_session_key. Use AskUserQuestion to confirm a dedicated session name first")
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
func Delivery(args map[string]any, sctx contract.ServerContext, executionMode, replyMode string, sessionTarget protocol.SessionTarget) (protocol.DeliveryTarget, error) {
	switch replyMode {
	case "":
		return protocol.DeliveryTarget{}, errors.New("reply_mode is required (none / execution / selected)")
	case "none":
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone}.Normalized(), nil
	case "execution":
		return executionReply(executionMode, args, sctx, sessionTarget)
	case "selected":
		to := argx.String(args, "selected_reply_session_key")
		if to == "" {
			return protocol.DeliveryTarget{}, errors.New("reply_mode=selected requires selected_reply_session_key. Use AskUserQuestion to confirm which existing session should receive the result")
		}
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeExplicit, Channel: "websocket", To: to}.Normalized(), nil
	default:
		return protocol.DeliveryTarget{}, fmt.Errorf("unsupported reply_mode: %s (allowed: none / execution / selected)", replyMode)
	}
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
	if (resolved == "temporary" || resolved == "dedicated") && sctx.SourceContextType != "chat" {
		return protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone}.Normalized(), nil
	}
	to := argx.FirstNonEmpty(argx.String(args, "selected_session_key"), sctx.CurrentSessionKey)
	if to == "" {
		return protocol.DeliveryTarget{}, errors.New("reply_mode=execution requires selected_session_key or an active current session. Use AskUserQuestion to confirm which execution session should receive the result")
	}
	return protocol.DeliveryTarget{Mode: protocol.DeliveryModeExplicit, Channel: "websocket", To: to}.Normalized(), nil
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
	contextLabel := sctx.CurrentAgentName
	if contextLabel == "" {
		contextLabel = agentID
	}
	source := protocol.Source{
		Kind:           protocol.SourceKindAgent,
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
