// Package builder 把 MCP 工具入参里的对象翻译成 automation 底层结构，
// 并复用底层的 Normalize + Validate。
package builder

import (
	"errors"
	"strings"

	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
)

// Schedule 把 schedule 对象翻译成底层 Schedule。
func Schedule(raw any) (automationsvc.Schedule, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.Schedule{}, errors.New("schedule must be an object")
	}
	schedule := automationsvc.Schedule{
		Kind:     argx.String(m, "kind"),
		Timezone: argx.String(m, "timezone"),
	}
	if v, ok := m["interval_seconds"]; ok {
		n := argx.Int(v)
		schedule.IntervalSeconds = &n
	}
	if v, ok := m["cron_expression"]; ok {
		s := strings.TrimSpace(argx.StringOf(v))
		schedule.CronExpression = &s
	}
	if v, ok := m["run_at"]; ok {
		s := strings.TrimSpace(argx.StringOf(v))
		schedule.RunAt = &s
	}
	normalized := schedule.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationsvc.Schedule{}, err
	}
	return normalized, nil
}

// SessionTarget 把 session_target 对象翻译成底层 SessionTarget。
// 当 kind=bound 且未填 bound_session_key 时，使用当前会话 fallback。
func SessionTarget(raw any, currentSessionKey string) (automationsvc.SessionTarget, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.SessionTarget{}, errors.New("session_target must be an object")
	}
	target := automationsvc.SessionTarget{
		Kind:            argx.String(m, "kind"),
		BoundSessionKey: argx.String(m, "bound_session_key"),
		NamedSessionKey: argx.String(m, "named_session_key"),
		WakeMode:        argx.String(m, "wake_mode"),
	}
	if target.Kind == automationsvc.SessionTargetBound && target.BoundSessionKey == "" && currentSessionKey != "" {
		target.BoundSessionKey = currentSessionKey
	}
	normalized := target.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationsvc.SessionTarget{}, err
	}
	return normalized, nil
}

// Delivery 把 delivery 对象翻译成底层 DeliveryTarget。
// 当 mode=explicit 且未填 to 时，使用当前会话 fallback 并补默认 channel=websocket。
func Delivery(raw any, currentSessionKey string) (automationsvc.DeliveryTarget, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.DeliveryTarget{}, errors.New("delivery must be an object")
	}
	delivery := automationsvc.DeliveryTarget{
		Mode:      argx.String(m, "mode"),
		Channel:   argx.String(m, "channel"),
		To:        argx.String(m, "to"),
		AccountID: argx.String(m, "account_id"),
		ThreadID:  argx.String(m, "thread_id"),
	}
	if delivery.Mode == automationsvc.DeliveryModeExplicit && delivery.To == "" && currentSessionKey != "" {
		if delivery.Channel == "" {
			delivery.Channel = "websocket"
		}
		delivery.To = currentSessionKey
	}
	normalized := delivery.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationsvc.DeliveryTarget{}, err
	}
	return normalized, nil
}

// Source 把 source 对象翻译成底层 Source。
func Source(raw any) (automationsvc.Source, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.Source{}, errors.New("source must be an object")
	}
	source := automationsvc.Source{
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
		return automationsvc.Source{}, err
	}
	return normalized, nil
}
