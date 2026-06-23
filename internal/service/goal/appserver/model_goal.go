package appserver

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ThreadGoalStatus 是 Codex app-server 协议中的 Goal 状态表示。
type ThreadGoalStatus string

const (
	ThreadGoalStatusActive        ThreadGoalStatus = "active"
	ThreadGoalStatusPaused        ThreadGoalStatus = "paused"
	ThreadGoalStatusBlocked       ThreadGoalStatus = "blocked"
	ThreadGoalStatusUsageLimited  ThreadGoalStatus = "usageLimited"
	ThreadGoalStatusBudgetLimited ThreadGoalStatus = "budgetLimited"
	ThreadGoalStatusComplete      ThreadGoalStatus = "complete"
)

// ThreadGoal 是 Codex app-server v2 的 Goal 投影。
type ThreadGoal struct {
	ThreadID        string           `json:"threadId"`
	Objective       string           `json:"objective"`
	Status          ThreadGoalStatus `json:"status"`
	TokenBudget     *int64           `json:"tokenBudget"`
	TokensUsed      int64            `json:"tokensUsed"`
	TimeUsedSeconds int64            `json:"timeUsedSeconds"`
	CreatedAt       int64            `json:"createdAt"`
	UpdatedAt       int64            `json:"updatedAt"`
}

// ThreadGoalSetParams 对齐 Codex app-server 的 thread/goal/set 参数。
type ThreadGoalSetParams struct {
	ThreadID    string                 `json:"threadId"`
	Objective   *string                `json:"objective,omitempty"`
	Status      *ThreadGoalStatus      `json:"status,omitempty"`
	TokenBudget protocol.OptionalInt64 `json:"tokenBudget,omitempty"`
}

// ThreadGoalSetResponse 对齐 Codex app-server 的 thread/goal/set 响应。
type ThreadGoalSetResponse struct {
	Goal ThreadGoal `json:"goal"`
}

// ThreadGoalGetParams 对齐 Codex app-server 的 thread/goal/get 参数。
type ThreadGoalGetParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadGoalGetResponse 对齐 Codex app-server 的 thread/goal/get 响应。
type ThreadGoalGetResponse struct {
	Goal *ThreadGoal `json:"goal"`
}

// ThreadGoalClearParams 对齐 Codex app-server 的 thread/goal/clear 参数。
type ThreadGoalClearParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadGoalClearResponse 对齐 Codex app-server 的 thread/goal/clear 响应。
type ThreadGoalClearResponse struct {
	Cleared bool `json:"cleared"`
}

// ThreadGoalUpdatedNotification 对齐 Codex app-server 的 thread/goal/updated 通知参数。
type ThreadGoalUpdatedNotification struct {
	ThreadID string     `json:"threadId"`
	TurnID   *string    `json:"turnId"`
	Goal     ThreadGoal `json:"goal"`
}

// ThreadGoalClearedNotification 对齐 Codex app-server 的 thread/goal/cleared 通知参数。
type ThreadGoalClearedNotification struct {
	ThreadID string `json:"threadId"`
}

// ThreadGoalFromGoal 把 Nexus Goal 转为 Codex app-server Goal 投影。
func ThreadGoalFromGoal(item protocol.Goal) ThreadGoal {
	return ThreadGoal{
		ThreadID:        item.SessionKey,
		Objective:       item.Objective,
		Status:          ThreadGoalStatusFromGoalStatus(item.Status),
		TokenBudget:     cloneInt64Pointer(item.TokenBudget),
		TokensUsed:      item.Usage.Total(),
		TimeUsedSeconds: item.TimeUsedSeconds,
		CreatedAt:       unixSeconds(item.CreatedAt),
		UpdatedAt:       unixSeconds(item.UpdatedAt),
	}
}

// ThreadGoalPointerFromGoal 返回可为空的 Codex app-server Goal 投影。
func ThreadGoalPointerFromGoal(item *protocol.Goal) *ThreadGoal {
	if item == nil {
		return nil
	}
	value := ThreadGoalFromGoal(*item)
	return &value
}

// ThreadGoalStatusFromGoalStatus 把 Nexus snake_case 状态转为 Codex camelCase 状态。
func ThreadGoalStatusFromGoalStatus(status protocol.GoalStatus) ThreadGoalStatus {
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusPaused:
		return ThreadGoalStatusPaused
	case protocol.GoalStatusBlocked:
		return ThreadGoalStatusBlocked
	case protocol.GoalStatusUsageLimited:
		return ThreadGoalStatusUsageLimited
	case protocol.GoalStatusBudgetLimited:
		return ThreadGoalStatusBudgetLimited
	case protocol.GoalStatusComplete:
		return ThreadGoalStatusComplete
	default:
		return ThreadGoalStatusActive
	}
}

// GoalStatusFromThreadGoalStatus 把 Codex app-server 状态转为 Nexus Goal 状态。
func GoalStatusFromThreadGoalStatus(status ThreadGoalStatus) (protocol.GoalStatus, bool) {
	switch status {
	case ThreadGoalStatusActive:
		return protocol.GoalStatusActive, true
	case ThreadGoalStatusPaused:
		return protocol.GoalStatusPaused, true
	case ThreadGoalStatusBlocked:
		return protocol.GoalStatusBlocked, true
	case ThreadGoalStatusUsageLimited:
		return protocol.GoalStatusUsageLimited, true
	case ThreadGoalStatusBudgetLimited:
		return protocol.GoalStatusBudgetLimited, true
	case ThreadGoalStatusComplete:
		return protocol.GoalStatusComplete, true
	default:
		return "", false
	}
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func unixSeconds(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}
