package protocol

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

// GoalStatus 表示会话 Goal 的生命周期状态。
type GoalStatus string

const (
	GoalStatusActive        GoalStatus = "active"
	GoalStatusPaused        GoalStatus = "paused"
	GoalStatusComplete      GoalStatus = "complete"
	GoalStatusBlocked       GoalStatus = "blocked"
	GoalStatusBudgetLimited GoalStatus = "budget_limited"
	GoalStatusUsageLimited  GoalStatus = "usage_limited"
)

// GoalUpdateSource 表示 Goal 状态变化来源。
type GoalUpdateSource string

const (
	GoalUpdateSourceUser     GoalUpdateSource = "user"
	GoalUpdateSourceModel    GoalUpdateSource = "model"
	GoalUpdateSourceSystem   GoalUpdateSource = "system"
	GoalUpdateSourceExternal GoalUpdateSource = "external"
)

const (
	GoalMetadataRoomGoalScope                         = "room_goal_scope"
	GoalMetadataRoomGoalLeadAgentID                   = "room_goal_lead_agent_id"
	GoalMetadataRoomGoalLeadAgentName                 = "room_goal_lead_agent_name"
	GoalMetadataRoomGoalLoopSlug                      = "room_goal_loop_slug"
	GoalMetadataRoomGoalLoopTitle                     = "room_goal_loop_title"
	GoalMetadataRoomGoalCollaborationRequired         = "room_goal_collaboration_required"
	GoalMetadataRoomGoalCollaborationObserved         = "room_goal_collaboration_observed"
	GoalMetadataRoomGoalCollaborationAgentID          = "room_goal_collaboration_agent_id"
	GoalMetadataRoomGoalCollaborationRoundID          = "room_goal_collaboration_round_id"
	GoalMetadataRoomGoalCollaborationObservedAt       = "room_goal_collaboration_observed_at"
	GoalMetadataRoomGoalCollaborationRequirementRound = "room_goal_collaboration_requirement_round_id"
)

// GoalUsage 记录 Goal 长程执行累计用量。
type GoalUsage struct {
	InputTokens              int64 `json:"input_tokens,omitempty"`
	OutputTokens             int64 `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	ReasoningTokens          int64 `json:"reasoning_tokens,omitempty"`
	TotalTokens              int64 `json:"total_tokens,omitempty"`
	RuntimeSeconds           int64 `json:"runtime_seconds,omitempty"`
}

// Total 返回可用于预算判断的 token 总量。
func (u GoalUsage) Total() int64 {
	return u.BudgetTokens()
}

// BudgetTokens 按 Codex Goal 口径统计预算 token：非缓存输入 token + 输出 token。
func (u GoalUsage) BudgetTokens() int64 {
	if u.hasTokenBreakdown() {
		input := u.InputTokens
		if input < 0 {
			input = 0
		}
		cachedInput := u.CacheReadInputTokens
		if cachedInput < 0 {
			cachedInput = 0
		}
		nonCachedInput := input - cachedInput
		if nonCachedInput < 0 {
			nonCachedInput = 0
		}
		output := u.OutputTokens
		if output < 0 {
			output = 0
		}
		return nonCachedInput + output
	}
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return 0
}

func (u GoalUsage) hasTokenBreakdown() bool {
	return u.InputTokens != 0 ||
		u.OutputTokens != 0 ||
		u.CacheCreationInputTokens != 0 ||
		u.CacheReadInputTokens != 0 ||
		u.ReasoningTokens != 0
}

// Add 合并 token usage。
func (u GoalUsage) Add(other GoalUsage) GoalUsage {
	totalTokens := u.BudgetTokens()
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheCreationInputTokens += other.CacheCreationInputTokens
	u.CacheReadInputTokens += other.CacheReadInputTokens
	u.ReasoningTokens += other.ReasoningTokens
	u.TotalTokens = totalTokens + other.BudgetTokens()
	u.RuntimeSeconds += other.RuntimeSeconds
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
	TimeUsedSeconds    int64          `json:"time_used_seconds,omitempty"`
	ContinuationCount  int            `json:"continuation_count"`
	EmptyProgressCount int            `json:"empty_progress_count"`
	Version            int64          `json:"version"`
	CreatedBy          string         `json:"created_by,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	BlockedAt          *time.Time     `json:"blocked_at,omitempty"`
	LastError          string         `json:"last_error,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// GoalMetadataString 从 Goal metadata 中读取字符串值。
func GoalMetadataString(metadata map[string]any, key string) string {
	value, ok := metadata[strings.TrimSpace(key)]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

// GoalMetadataBool 从 Goal metadata 中读取布尔值。
func GoalMetadataBool(metadata map[string]any, key string) bool {
	value, ok := metadata[strings.TrimSpace(key)]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return false
	}
}

// GoalRoomLeadAgentID 返回 Room Goal 的负责人 Agent。
func GoalRoomLeadAgentID(goal Goal) string {
	return GoalMetadataString(goal.Metadata, GoalMetadataRoomGoalLeadAgentID)
}

// GoalRoomLeadAgentName 返回 Room Goal 的负责人展示名。
func GoalRoomLeadAgentName(goal Goal) string {
	return GoalMetadataString(goal.Metadata, GoalMetadataRoomGoalLeadAgentName)
}

// GoalRoomCollaborationRequired 判断 Room Goal 是否要求非负责人可见协作。
func GoalRoomCollaborationRequired(goal Goal) bool {
	return GoalMetadataBool(goal.Metadata, GoalMetadataRoomGoalCollaborationRequired)
}

// GoalRoomCollaborationObserved 判断 Room Goal 是否已有非负责人可见协作证据。
func GoalRoomCollaborationObserved(goal Goal) bool {
	return GoalMetadataBool(goal.Metadata, GoalMetadataRoomGoalCollaborationObserved)
}

// RemainingTokens 返回剩余 token 预算；没有预算时返回 nil。
func (g Goal) RemainingTokens() *int64 {
	if g.TokenBudget == nil || *g.TokenBudget <= 0 {
		return nil
	}
	remaining := *g.TokenBudget - g.Usage.Total()
	if remaining < 0 {
		remaining = 0
	}
	return &remaining
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

// OptionalInt64 表示 JSON 字段的三态：缺省、null、整数值。
type OptionalInt64 struct {
	Present bool
	Value   *int64
}

// UnmarshalJSON 记录字段是否出现，并保留 null 与整数值的差异。
func (v *OptionalInt64) UnmarshalJSON(data []byte) error {
	v.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		v.Value = nil
		return nil
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

// GoalContinuation 表示一次由系统触发的隐藏 Goal 续跑输入。
type GoalContinuation struct {
	Goal           Goal              `json:"goal"`
	RoundID        string            `json:"round_id"`
	Prompt         string            `json:"prompt"`
	HiddenFromUser bool              `json:"hidden_from_user"`
	Synthetic      bool              `json:"synthetic"`
	Purpose        string            `json:"purpose"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// CreateGoalRequest 表示创建 Goal 的请求。
type CreateGoalRequest struct {
	SessionKey  string         `json:"session_key"`
	Objective   string         `json:"objective"`
	TokenBudget *int64         `json:"token_budget,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty"`
	RoundID     string         `json:"round_id,omitempty"`
	OwnerUserID string         `json:"owner_user_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// UpdateGoalRequest 表示更新 Goal 的请求。
type UpdateGoalRequest struct {
	Objective   *string        `json:"objective,omitempty"`
	TokenBudget OptionalInt64  `json:"token_budget,omitempty"`
	OwnerUserID string         `json:"owner_user_id,omitempty"`
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
	case GoalStatusBudgetLimited:
		return GoalStatusBudgetLimited
	case GoalStatusUsageLimited:
		return GoalStatusUsageLimited
	case GoalStatus("cleared"):
		return GoalStatusComplete
	default:
		return GoalStatusActive
	}
}

// IsCurrentGoalStatus 判断状态是否属于当前 Goal。
func IsCurrentGoalStatus(status GoalStatus) bool {
	switch NormalizeGoalStatus(status) {
	case GoalStatusActive, GoalStatusPaused, GoalStatusBlocked, GoalStatusBudgetLimited, GoalStatusUsageLimited:
		return true
	default:
		return false
	}
}

// IsRuntimeGoalStatus 判断状态是否应注入运行时上下文。
func IsRuntimeGoalStatus(status GoalStatus) bool {
	switch NormalizeGoalStatus(status) {
	case GoalStatusActive:
		return true
	default:
		return false
	}
}

// IsRuntimeAccountingGoalStatus 判断状态是否应作为运行中 round 的 Goal usage 目标。
func IsRuntimeAccountingGoalStatus(status GoalStatus) bool {
	switch NormalizeGoalStatus(status) {
	case GoalStatusActive, GoalStatusBudgetLimited:
		return true
	default:
		return false
	}
}
