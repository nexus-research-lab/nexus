// Package semantic 承载页面语义(execution_mode/reply_mode) 与 Source 到
// automation 底层 SessionTarget / DeliveryTarget / Source 的翻译与校验。
package semantic

import (
	"errors"
	"fmt"
	"strings"

	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/builder"
)

// SessionTarget 优先使用显式 session_target，否则按 execution_mode 推导。
func SessionTarget(args map[string]any, sctx contract.ServerContext, executionMode string) (automationsvc.SessionTarget, error) {
	if raw, ok := args["session_target"]; ok {
		return builder.SessionTarget(raw, sctx.CurrentSessionKey)
	}
	return sessionTargetFromMode(executionMode, args, sctx)
}

func sessionTargetFromMode(executionMode string, args map[string]any, sctx contract.ServerContext) (automationsvc.SessionTarget, error) {
	switch executionMode {
	case "":
		return automationsvc.SessionTarget{}, errors.New("session_target or execution_mode is required")
	case "main":
		return automationsvc.SessionTarget{Kind: automationsvc.SessionTargetMain, WakeMode: automationsvc.WakeModeNextHeartbeat}.Normalized(), nil
	case "existing", "current_chat":
		bound := argx.FirstNonEmpty(argx.String(args, "selected_session_key"), sctx.CurrentSessionKey)
		if bound == "" {
			return automationsvc.SessionTarget{}, errors.New("execution_mode=existing requires selected_session_key or an active current session. Use AskUserQuestion to confirm which existing session should execute the task")
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
		return automationsvc.SessionTarget{}, fmt.Errorf("unsupported execution_mode: %s", executionMode)
	}
}

// Delivery 优先使用显式 delivery，否则按 reply_mode 推导。
func Delivery(args map[string]any, sctx contract.ServerContext, executionMode, replyMode string, sessionTarget automationsvc.SessionTarget) (automationsvc.DeliveryTarget, error) {
	if raw, ok := args["delivery"]; ok {
		return builder.Delivery(raw, sctx.CurrentSessionKey)
	}
	return deliveryFromMode(replyMode, executionMode, args, sctx, sessionTarget)
}

func deliveryFromMode(replyMode, executionMode string, args map[string]any, sctx contract.ServerContext, sessionTarget automationsvc.SessionTarget) (automationsvc.DeliveryTarget, error) {
	switch replyMode {
	case "":
		return automationsvc.DeliveryTarget{}, errors.New("delivery or reply_mode is required")
	case "none":
		return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeNone}.Normalized(), nil
	case "current_chat":
		if sctx.CurrentSessionKey == "" {
			return automationsvc.DeliveryTarget{}, errors.New("reply_mode=current_chat requires an active current session. Use AskUserQuestion to confirm which existing session should receive the result")
		}
		return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeExplicit, Channel: "websocket", To: sctx.CurrentSessionKey}.Normalized(), nil
	case "execution":
		return executionReply(executionMode, args, sctx, sessionTarget)
	case "selected":
		to := argx.String(args, "selected_reply_session_key")
		if to == "" {
			return automationsvc.DeliveryTarget{}, errors.New("reply_mode=selected requires selected_reply_session_key. Use AskUserQuestion to confirm which existing session should receive the result")
		}
		return automationsvc.DeliveryTarget{Mode: automationsvc.DeliveryModeExplicit, Channel: "websocket", To: to}.Normalized(), nil
	default:
		return automationsvc.DeliveryTarget{}, fmt.Errorf("unsupported reply_mode: %s", replyMode)
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
func ValidatePage(target automationsvc.SessionTarget, delivery automationsvc.DeliveryTarget, executionMode, replyMode string) error {
	if delivery.Mode == automationsvc.DeliveryModeLast {
		return errors.New("delivery.mode=last is not supported by the scheduled-task page semantics. Use AskUserQuestion and choose none/execution/current_chat/selected explicitly")
	}
	execMode := strings.TrimSpace(executionMode)
	rplMode := strings.TrimSpace(replyMode)
	if execMode == "main" && rplMode != "" && rplMode != "none" {
		return errors.New("execution_mode=main does not support reply_mode under page semantics. To run independently and send the result back here, use temporary + current_chat")
	}
	if target.Kind == automationsvc.SessionTargetMain && delivery.Mode != automationsvc.DeliveryModeNone {
		return errors.New("session_target.kind=main cannot be combined with delivery.mode!=none under page semantics. To run independently and send the result back here, use temporary + current_chat")
	}
	return nil
}

// Source 把工具入参里的 source 与当前 ServerContext 合并，缺字段自动用上下文补齐。
func Source(raw any, sctx contract.ServerContext, agentID string) automationsvc.Source {
	contextLabel := sctx.CurrentAgentName
	if contextLabel == "" {
		contextLabel = agentID
	}
	defaults := automationsvc.Source{
		Kind:           automationsvc.SourceKindAgent,
		CreatorAgentID: sctx.CurrentAgentID,
		ContextType:    "agent",
		ContextID:      agentID,
		ContextLabel:   contextLabel,
		SessionKey:     sctx.CurrentSessionKey,
		SessionLabel:   argx.FirstNonEmpty(sctx.CurrentSessionLabel, sessionLabelFallback(sctx.CurrentSessionKey)),
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return defaults.Normalized()
	}
	source := automationsvc.Source{
		Kind:           argx.FirstNonEmpty(argx.String(m, "kind"), defaults.Kind),
		CreatorAgentID: argx.FirstNonEmpty(argx.String(m, "creator_agent_id"), defaults.CreatorAgentID),
		ContextType:    argx.FirstNonEmpty(argx.String(m, "context_type"), defaults.ContextType),
		ContextID:      argx.FirstNonEmpty(argx.String(m, "context_id"), defaults.ContextID),
		ContextLabel:   argx.FirstNonEmpty(argx.String(m, "context_label"), defaults.ContextLabel),
		SessionKey:     argx.FirstNonEmpty(argx.String(m, "session_key"), defaults.SessionKey),
		SessionLabel:   argx.FirstNonEmpty(argx.String(m, "session_label"), defaults.SessionLabel),
	}
	return source.Normalized()
}

func sessionLabelFallback(sessionKey string) string {
	if sessionKey == "" {
		return ""
	}
	return "当前对话"
}
