package protocol

import (
	"strings"
	"time"
)

// GoalStatus 表示会话 Goal 的生命周期状态。
type GoalStatus string

const (
	GoalStatusActive   GoalStatus = "active"
	GoalStatusPaused   GoalStatus = "paused"
	GoalStatusComplete GoalStatus = "complete"
	GoalStatusBlocked  GoalStatus = "blocked"
	GoalStatusCleared  GoalStatus = "cleared"
)

// GoalUpdateSource 表示 Goal 状态变化来源。
type GoalUpdateSource string

const (
	GoalUpdateSourceUser   GoalUpdateSource = "user"
	GoalUpdateSourceModel  GoalUpdateSource = "model"
	GoalUpdateSourceSystem GoalUpdateSource = "system"
)

// GoalUsage 记录 Goal 长程执行累计用量。
type GoalUsage struct {
	InputTokens              int64 `json:"input_tokens,omitempty"`
	OutputTokens             int64 `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	ReasoningTokens          int64 `json:"reasoning_tokens,omitempty"`
	TotalTokens              int64 `json:"total_tokens,omitempty"`
}

// Add 合并 token usage。
func (u GoalUsage) Add(other GoalUsage) GoalUsage {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheCreationInputTokens += other.CacheCreationInputTokens
	u.CacheReadInputTokens += other.CacheReadInputTokens
	u.ReasoningTokens += other.ReasoningTokens
	u.TotalTokens += other.TotalTokens
	if u.TotalTokens == 0 {
		u.TotalTokens = u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens + u.ReasoningTokens
	}
	return u
}

// Goal 表示一个 session 的当前长程目标。
type Goal struct {
	ID                 string         `json:"id"`
	SessionKey         string         `json:"session_key"`
	Objective          string         `json:"objective"`
	Status             GoalStatus     `json:"status"`
	TokenBudget        *int64         `json:"token_budget,omitempty"`
	Usage              GoalUsage      `json:"usage"`
	ContinuationCount  int            `json:"continuation_count"`
	EmptyProgressCount int            `json:"empty_progress_count"`
	Version            int64          `json:"version"`
	CreatedBy          string         `json:"created_by,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	BlockedAt          *time.Time     `json:"blocked_at,omitempty"`
	ClearedAt          *time.Time     `json:"cleared_at,omitempty"`
	LastError          string         `json:"last_error,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// GoalEvent 表示 Goal 审计事件。
type GoalEvent struct {
	ID         string           `json:"id"`
	GoalID     string           `json:"goal_id"`
	SessionKey string           `json:"session_key"`
	EventType  string           `json:"event_type"`
	Source     GoalUpdateSource `json:"source"`
	RoundID    string           `json:"round_id,omitempty"`
	Payload    map[string]any   `json:"payload,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

// GoalCheckpoint 表示 Goal 长程执行 checkpoint。
type GoalCheckpoint struct {
	ID                string    `json:"id"`
	GoalID            string    `json:"goal_id"`
	SessionKey        string    `json:"session_key"`
	Summary           string    `json:"summary"`
	ContinuationCount int       `json:"continuation_count"`
	Usage             GoalUsage `json:"usage"`
	CreatedAt         time.Time `json:"created_at"`
}

// CreateGoalRequest 表示创建 Goal 的请求。
type CreateGoalRequest struct {
	SessionKey  string         `json:"session_key"`
	Objective   string         `json:"objective"`
	TokenBudget *int64         `json:"token_budget,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// UpdateGoalRequest 表示更新 Goal 的请求。
type UpdateGoalRequest struct {
	Objective   *string        `json:"objective,omitempty"`
	TokenBudget *int64         `json:"token_budget,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// CompleteGoalRequest 表示完成 Goal 的请求。
type CompleteGoalRequest struct {
	Summary string `json:"summary,omitempty"`
	RoundID string `json:"round_id,omitempty"`
}

// BlockGoalRequest 表示阻塞 Goal 的请求。
type BlockGoalRequest struct {
	Reason      string `json:"reason"`
	NeededInput string `json:"needed_input,omitempty"`
	RoundID     string `json:"round_id,omitempty"`
}

// GoalEventEnvelope 构造 WebSocket Goal 事件。
func GoalEventEnvelope(sessionKey string, eventType EventType, goal Goal, payload map[string]any) EventMessage {
	data := map[string]any{"goal": goal}
	for key, value := range payload {
		data[key] = value
	}
	event := NewEvent(eventType, data)
	event.SessionKey = strings.TrimSpace(sessionKey)
	return event
}

// NormalizeGoalStatus 规范化 Goal 状态。
func NormalizeGoalStatus(status GoalStatus) GoalStatus {
	switch GoalStatus(strings.TrimSpace(string(status))) {
	case GoalStatusPaused:
		return GoalStatusPaused
	case GoalStatusComplete:
		return GoalStatusComplete
	case GoalStatusBlocked:
		return GoalStatusBlocked
	case GoalStatusCleared:
		return GoalStatusCleared
	default:
		return GoalStatusActive
	}
}

// IsCurrentGoalStatus 判断状态是否属于当前 Goal。
func IsCurrentGoalStatus(status GoalStatus) bool {
	switch NormalizeGoalStatus(status) {
	case GoalStatusActive, GoalStatusPaused, GoalStatusBlocked:
		return true
	default:
		return false
	}
}
