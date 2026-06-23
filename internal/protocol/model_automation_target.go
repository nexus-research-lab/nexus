package protocol

import (
	"errors"
	"strings"
)

// SessionTarget 表示执行目标会话。
type SessionTarget struct {
	Kind            string `json:"kind"`
	BoundSessionKey string `json:"bound_session_key,omitempty"`
	NamedSessionKey string `json:"named_session_key,omitempty"`
	WakeMode        string `json:"wake_mode,omitempty"`
}

// Validate 校验会话目标。
func (t SessionTarget) Validate() error {
	kind := strings.TrimSpace(t.Kind)
	boundSessionKey := strings.TrimSpace(t.BoundSessionKey)
	namedSessionKey := strings.TrimSpace(t.NamedSessionKey)
	wakeMode := strings.TrimSpace(t.WakeMode)
	if wakeMode == "" {
		wakeMode = WakeModeNextHeartbeat
	}
	switch wakeMode {
	case WakeModeNow, WakeModeNextHeartbeat:
	default:
		return errors.New("wake_mode must be one of now, next-heartbeat")
	}

	switch kind {
	case SessionTargetIsolated, SessionTargetMain:
		if boundSessionKey != "" || namedSessionKey != "" {
			return errors.New("bound_session_key and named_session_key must be empty for isolated/main target")
		}
	case SessionTargetBound:
		if boundSessionKey == "" {
			return errors.New("bound_session_key is required when session_target.kind is bound")
		}
		if _, err := RequireStructuredSessionKey(boundSessionKey); err != nil {
			return err
		}
	case SessionTargetNamed:
		if namedSessionKey == "" {
			return errors.New("named_session_key is required when session_target.kind is named")
		}
		if strings.EqualFold(namedSessionKey, "main") {
			return errors.New("named_session_key 'main' is reserved")
		}
	default:
		return errors.New("session_target.kind must be one of isolated, main, bound, named")
	}
	return nil
}

// Normalized 返回带默认值的会话目标副本。
func (t SessionTarget) Normalized() SessionTarget {
	result := t
	result.Kind = strings.TrimSpace(result.Kind)
	if result.Kind == "" {
		result.Kind = SessionTargetIsolated
	}
	result.BoundSessionKey = strings.TrimSpace(result.BoundSessionKey)
	result.NamedSessionKey = strings.TrimSpace(result.NamedSessionKey)
	result.WakeMode = strings.TrimSpace(result.WakeMode)
	if result.WakeMode == "" {
		result.WakeMode = WakeModeNextHeartbeat
	}
	return result
}

// DeliveryTarget 表示自动化外部投递目标。
type DeliveryTarget struct {
	Mode      string `json:"mode"`
	Channel   string `json:"channel,omitempty"`
	To        string `json:"to,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
}

// Source 表示任务来源元数据。
type Source struct {
	Kind           string `json:"kind"`
	CreatorAgentID string `json:"creator_agent_id,omitempty"`
	ContextType    string `json:"context_type,omitempty"`
	ContextID      string `json:"context_id,omitempty"`
	ContextLabel   string `json:"context_label,omitempty"`
	SessionKey     string `json:"session_key,omitempty"`
	SessionLabel   string `json:"session_label,omitempty"`
}

// Validate 校验投递目标。
func (d DeliveryTarget) Validate() error {
	switch strings.TrimSpace(d.Mode) {
	case "", DeliveryModeNone, DeliveryModeLast, DeliveryModeExplicit:
		return nil
	default:
		return errors.New("delivery.mode must be one of none, last, explicit")
	}
}

// Normalized 返回带默认值的投递目标副本。
func (d DeliveryTarget) Normalized() DeliveryTarget {
	result := d
	result.Mode = strings.TrimSpace(result.Mode)
	if result.Mode == "" {
		result.Mode = DeliveryModeNone
	}
	result.Channel = strings.TrimSpace(result.Channel)
	result.To = strings.TrimSpace(result.To)
	result.AccountID = strings.TrimSpace(result.AccountID)
	result.ThreadID = strings.TrimSpace(result.ThreadID)
	return result
}

// Validate 校验任务来源。
func (s Source) Validate() error {
	contextID := strings.TrimSpace(s.ContextID)
	contextLabel := strings.TrimSpace(s.ContextLabel)
	switch strings.TrimSpace(s.Kind) {
	case "", SourceKindUserPage, SourceKindAgent, SourceKindCLI, SourceKindSystem:
	default:
		return errors.New("source.kind must be one of user_page, agent, cli, system")
	}
	contextType := strings.TrimSpace(s.ContextType)
	switch contextType {
	case "", "agent", "room":
	default:
		return errors.New("source.context_type must be one of agent, room")
	}
	if contextType == "" {
		if contextID != "" || contextLabel != "" {
			return errors.New("context_type is required when context_id or context_label is provided")
		}
	} else if contextID == "" {
		return errors.New("context_id is required when context_type is provided")
	}
	if strings.TrimSpace(s.SessionKey) != "" {
		if _, err := RequireStructuredSessionKey(s.SessionKey); err != nil {
			return err
		}
	}
	return nil
}

// Normalized 返回带默认值的来源副本。
func (s Source) Normalized() Source {
	result := s
	result.Kind = strings.TrimSpace(result.Kind)
	if result.Kind == "" {
		result.Kind = SourceKindSystem
	}
	result.CreatorAgentID = strings.TrimSpace(result.CreatorAgentID)
	result.ContextType = strings.TrimSpace(result.ContextType)
	result.ContextID = strings.TrimSpace(result.ContextID)
	result.ContextLabel = strings.TrimSpace(result.ContextLabel)
	result.SessionKey = strings.TrimSpace(result.SessionKey)
	result.SessionLabel = strings.TrimSpace(result.SessionLabel)
	return result
}
