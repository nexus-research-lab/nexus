// INPUT: 用户级 Echo 开关、内部策略、DM 空闲锚点与后台执行状态。
// OUTPUT: Echo 内部策略、DM 空闲锚点与后台执行状态。
// POS: Echo 的领域合同；不依赖 HTTP、数据库或 runtime 实现。
package echo

import (
	"errors"
	"strings"
	"time"
)

const (
	TriggerConversationIdle = "conversation_idle"

	StatusScheduled  = "scheduled"
	StatusEvaluating = "evaluating"
	StatusRunning    = "running"
	StatusCommitting = "committing"
	StatusDelivered  = "delivered"
	StatusSuppressed = "suppressed"
	StatusCancelled  = "cancelled"
	StatusFailed     = "failed"

	SessionModeInherit  = "inherit"
	SessionModeEnabled  = "enabled"
	SessionModeDisabled = "disabled"
)

var (
	ErrAttemptNotAdmitted = errors.New("Echo 尝试已失效")
	ErrUnsupportedSession = errors.New("Echo 仅支持 WebSocket 私聊")
	ErrInvalidSessionMode = errors.New("Echo 会话设置无效")
)

// Settings 是用户可见的 Echo 全局开关。
type Settings struct {
	Enabled bool `json:"enabled"`
}

// Policy 表示 Echo 使用的内部主动跟进策略。
type Policy struct {
	Enabled          bool
	Timezone         string
	ActiveStart      string
	ActiveEnd        string
	IdleDelaySeconds int
	CooldownSeconds  int
	DailyLimit       int
}

// DefaultPolicy 返回关闭状态下的产品默认值。
func DefaultPolicy(timezone string) Policy {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	return Policy{
		Timezone:         timezone,
		ActiveStart:      "09:00",
		ActiveEnd:        "22:00",
		IdleDelaySeconds: 6 * 60 * 60,
		CooldownSeconds:  24 * 60 * 60,
		DailyLimit:       2,
	}
}

// Attempt 表示一次由成功 DM round 锚定的主动跟进尝试。
type Attempt struct {
	AttemptID          string     `json:"attempt_id"`
	OwnerUserID        string     `json:"-"`
	AgentID            string     `json:"agent_id"`
	SessionKey         string     `json:"session_key"`
	TriggerKind        string     `json:"trigger_kind"`
	AnchorRoundID      string     `json:"anchor_round_id"`
	AnchorMessageID    string     `json:"anchor_message_id,omitempty"`
	AnchorFinishedAt   time.Time  `json:"anchor_finished_at"`
	DueAt              time.Time  `json:"due_at"`
	ExpiresAt          time.Time  `json:"expires_at"`
	Status             string     `json:"status"`
	RuntimeRoundID     string     `json:"runtime_round_id,omitempty"`
	DecisionReason     string     `json:"decision_reason,omitempty"`
	Focus              string     `json:"focus,omitempty"`
	ErrorCode          string     `json:"error_code,omitempty"`
	DeliveredMessageID string     `json:"delivered_message_id,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// SessionOverride 表示单个 DM 是否覆盖用户级默认策略。
type SessionOverride struct {
	Mode             string `json:"mode"`
	EffectiveEnabled bool   `json:"enabled"`
}

// NormalizeSessionMode 归一化会话级覆盖。
func NormalizeSessionMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "", SessionModeInherit:
		return SessionModeInherit, nil
	case SessionModeEnabled, SessionModeDisabled:
		return mode, nil
	default:
		return "", ErrInvalidSessionMode
	}
}

// ParseClock 把 HH:mm 转成当天分钟数。
func ParseClock(value string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}
