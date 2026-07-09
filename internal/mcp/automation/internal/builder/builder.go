// Package builder 把 MCP 工具入参里的对象翻译成 automation 底层结构，
// 并复用底层的 Normalize + Validate。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package builder

import (
	"errors"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

// SessionTarget 把 session_target 对象翻译成底层 SessionTarget。
// 当 kind=bound 且未填 bound_session_key 时，使用当前会话 fallback。
func SessionTarget(raw any, currentSessionKey string) (automationdomain.SessionTarget, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationdomain.SessionTarget{}, errors.New("session_target must be an object")
	}
	target := automationdomain.SessionTarget{
		Kind:            argx.String(m, "kind"),
		BoundSessionKey: argx.String(m, "bound_session_key"),
		NamedSessionKey: argx.String(m, "named_session_key"),
		WakeMode:        argx.String(m, "wake_mode"),
	}
	if target.Kind == automationdomain.SessionTargetBound && target.BoundSessionKey == "" && currentSessionKey != "" {
		target.BoundSessionKey = currentSessionKey
	}
	normalized := target.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationdomain.SessionTarget{}, err
	}
	return normalized, nil
}

// Delivery 把 delivery 对象翻译成底层 DeliveryTarget。
// 当 mode=explicit 且未填 to 时，使用当前会话 fallback 并补默认 channel=websocket。
func Delivery(raw any, currentSessionKey string) (automationdomain.DeliveryTarget, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationdomain.DeliveryTarget{}, errors.New("delivery must be an object")
	}
	delivery := automationdomain.DeliveryTarget{
		Mode:      argx.String(m, "mode"),
		Channel:   argx.String(m, "channel"),
		To:        argx.String(m, "to"),
		AccountID: argx.String(m, "account_id"),
		ThreadID:  argx.String(m, "thread_id"),
	}
	if delivery.Mode == automationdomain.DeliveryModeExplicit && delivery.To == "" && currentSessionKey != "" {
		if delivery.Channel == "" {
			delivery.Channel = "websocket"
		}
		delivery.To = currentSessionKey
	}
	normalized := delivery.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationdomain.DeliveryTarget{}, err
	}
	return normalized, nil
}

// Source 把 source 对象翻译成底层 Source。
func Source(raw any) (automationdomain.Source, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationdomain.Source{}, errors.New("source must be an object")
	}
	source := automationdomain.Source{
		Kind:           argx.String(m, "kind"),
		CreatorAgentID: argx.String(m, "creator_agent_id"),
		ContextType:    argx.String(m, "context_type"),
		ContextID:      argx.String(m, "context_id"),
		ContextLabel:   argx.String(m, "context_label"),
		SessionKey:     argx.String(m, "session_key"),
		SessionLabel:   argx.String(m, "session_label"),
	}
	normalized := source.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationdomain.Source{}, err
	}
	return normalized, nil
}
