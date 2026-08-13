// INPUT: 跨 HTTP/WS/runtime 的 Goal 状态、请求、host Goal command、最终 usage fence 与 continuation 数据。
// OUTPUT: Goal 领域协议、独立 Goal 控制请求、Goal-only/managed Execution mode、server-derived continuation/binding 只读投影、按 ID 查询的 usage report、Room creator/lead 权限身份及归一化常量。
// POS: Goal 前后端与运行时共享的协议真相源。
package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// GoalContinuationState separates Goal lifecycle from the automatic
// continuation controller. An active Goal may be recovering or suspended
// without pretending that the user paused the Goal itself.
type GoalContinuationState string

const (
	GoalContinuationStateInactive   GoalContinuationState = "inactive"
	GoalContinuationStateReady      GoalContinuationState = "ready"
	GoalContinuationStateRecovering GoalContinuationState = "recovering"
	GoalContinuationStateSuspended  GoalContinuationState = "suspended"

	// GoalContinuationSuppressionThreshold is the consecutive no-progress
	// count at which automatic continuation stops. The preceding empty turn is
	// the one server-owned recovery opportunity.
	GoalContinuationSuppressionThreshold = 2
)

// GoalUpdateSource 表示 Goal 状态变化来源。
type GoalUpdateSource string

const (
	GoalUpdateSourceUser     GoalUpdateSource = "user"
	GoalUpdateSourceModel    GoalUpdateSource = "model"
	GoalUpdateSourceSystem   GoalUpdateSource = "system"
	GoalUpdateSourceExternal GoalUpdateSource = "external"
)

// GoalCollaborationBinding attributes a Room handoff to the exact Goal
// revision that requested it. It is host scheduling provenance only and must
// never be interpreted as Goal mutation authority by a target Agent round.
type GoalCollaborationBinding struct {
	GoalID            string `json:"goal_id"`
	ObjectiveRevision int64  `json:"objective_revision"`
}

// NormalizeGoalCollaborationBinding returns one canonical attribution value.
func NormalizeGoalCollaborationBinding(binding *GoalCollaborationBinding) *GoalCollaborationBinding {
	if binding == nil {
		return nil
	}
	result := &GoalCollaborationBinding{
		GoalID:            strings.TrimSpace(binding.GoalID),
		ObjectiveRevision: binding.ObjectiveRevision,
	}
	if !result.Valid() {
		return nil
	}
	return result
}

// Valid reports whether the binding can safely fence one current Goal revision.
func (b GoalCollaborationBinding) Valid() bool {
	return strings.TrimSpace(b.GoalID) != "" && b.ObjectiveRevision > 0
}

// GoalExecutionBindingState separates a future Execution reservation from a
// prepared binding and a binding confirmed after the authoritative Execution
// mutation. Standalone and conflict are resolver results and are never stored
// in Goal metadata.
type GoalExecutionBindingState string

const (
	GoalExecutionBindingStateStandalone GoalExecutionBindingState = "standalone"
	GoalExecutionBindingStateReserved   GoalExecutionBindingState = "reserved"
	GoalExecutionBindingStatePending    GoalExecutionBindingState = "pending"
	GoalExecutionBindingStateConfirmed  GoalExecutionBindingState = "confirmed"
	GoalExecutionBindingStateConflict   GoalExecutionBindingState = "conflict"
)

// GoalExecutionMode records whether a Goal is still independent or has
// explicitly entered the managed WorkGraph lifecycle. It is orthogonal to the
// binding phase: managed mode may be reserved, pending, or confirmed.
type GoalExecutionMode string

const (
	GoalExecutionModeGoalOnly GoalExecutionMode = "goal_only"
	GoalExecutionModeManaged  GoalExecutionMode = "managed"
)

// GoalExecutionBindingResolution is the shared Goal/Execution binding read
// model. ReservedExecutionID is provenance only; ExecutionID is populated only
// for an exact authoritative binding.
type GoalExecutionBindingResolution struct {
	State               GoalExecutionBindingState `json:"state"`
	ReservedExecutionID string                    `json:"reserved_execution_id,omitempty"`
	ExecutionID         string                    `json:"execution_id,omitempty"`
}

// GoalExecutionBindingView is the owner-scoped HTTP read model. It never
// exposes reservation provenance; ExecutionID is present only after the
// server proves one exact confirmed bilateral binding.
type GoalExecutionBindingView struct {
	State       GoalExecutionBindingState `json:"state"`
	ExecutionID string                    `json:"execution_id,omitempty"`
}

const (
	GoalMetadataRoomGoalScope          = "room_goal_scope"
	GoalMetadataRoomGoalCreatorAgentID = "room_goal_creator_agent_id"
	GoalMetadataRoomGoalLeadAgentID    = "room_goal_lead_agent_id"
	GoalMetadataRoomGoalLeadAgentName  = "room_goal_lead_agent_name"
	GoalMetadataRoomGoalLoopSlug       = "room_goal_loop_slug"
	GoalMetadataRoomGoalLoopTitle      = "room_goal_loop_title"
	// GoalMetadataRoomGoalCollaborationRequired is retained for historical
	// decoding only. New code neither writes nor treats it as a completion gate.
	GoalMetadataRoomGoalCollaborationRequired = "room_goal_collaboration_required"
	// Collaboration evidence is optional audit context and monotonic for one durable Goal ID. The stored
	// round/agent/revision provenance fences late writes; objective retarget does
	// not erase an already observed public non-lead contribution.
	GoalMetadataRoomGoalCollaborationObserved         = "room_goal_collaboration_observed"
	GoalMetadataRoomGoalCollaborationAgentID          = "room_goal_collaboration_agent_id"
	GoalMetadataRoomGoalCollaborationRoundID          = "room_goal_collaboration_round_id"
	GoalMetadataRoomGoalCollaborationObservedAt       = "room_goal_collaboration_observed_at"
	GoalMetadataRoomGoalCollaborationRequirementRound = "room_goal_collaboration_requirement_round_id"
	GoalMetadataObjectiveRevision                     = "objective_revision"
	// GoalMetadataSourceObjective preserves the user's original Goal intent when
	// best-effort normalization expands it into the canonical objective.
	GoalMetadataSourceObjective     = "source_objective"
	GoalMetadataObjectiveNormalized = "objective_normalized"
	// GoalMetadataOwnerUserID is server-owned authorization provenance for
	// owner-scoped Goal reads and mutations. Request metadata cannot replace it.
	GoalMetadataOwnerUserID = "owner_user_id"
	GoalMetadataExecutionID = "execution_id"
	// GoalMetadataExecutionMode distinguishes new Goal-only records from legacy
	// explicit Goals whose command identity implied an Execution reservation.
	// User metadata cannot write this field.
	GoalMetadataExecutionMode = "execution_mode"
	// GoalMetadataExecutionBindingState is server-owned. Only reserved,
	// pending and confirmed are persisted; standalone/conflict are derived by
	// the binding resolver.
	GoalMetadataExecutionBindingState = "execution_binding_state"
	GoalMetadataPromotionCommand      = "promotion_command"
	GoalMetadataActivationOrigin      = "activation_origin"
	GoalMetadataActivationReason      = "activation_reason"
	GoalMetadataCompletionCriteria    = "completion_criteria"
	GoalMetadataObjectiveAlignment    = "objective_alignment"
	GoalMetadataExplicitCommand       = "explicit_goal_command"
	// GoalMetadataObjectiveTransition is server-owned durable state for a
	// Goal objective revision rebase. User metadata updates must never replace
	// or remove it.
	GoalMetadataObjectiveTransition = "objective_transition"
)

// GoalUsage 记录 Goal 长程执行累计用量。
//
// TotalTokens 是旧客户端使用的预算口径别名；实际处理总量由
// ActualTotalTokens 单独承载，避免缓存 token 被预算口径覆盖。
type GoalUsage struct {
	InputTokens              int64 `json:"input_tokens,omitempty"`
	OutputTokens             int64 `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	ReasoningTokens          int64 `json:"reasoning_tokens,omitempty"`
	TotalTokens              int64 `json:"total_tokens,omitempty"`
	BudgetTotalTokens        int64 `json:"budget_tokens"`
	ActualTotalTokens        int64 `json:"actual_tokens"`
	ActualTokensEstimated    bool  `json:"actual_tokens_estimated,omitempty"`
	RuntimeSeconds           int64 `json:"runtime_seconds,omitempty"`
	// *TotalKnown 只在进程内区分“权威零增量”和“缺少显式总量”；
	// JSON/SQL 仍由上面的稳定字段承载。
	BudgetTotalKnown bool `json:"-"`
	ActualTotalKnown bool `json:"-"`
}

// Total 返回旧版预算口径总量。
// Deprecated: 新代码应显式调用 BudgetTokens 或 ActualTokens。
func (u GoalUsage) Total() int64 {
	return u.BudgetTokens()
}

// BudgetTokens 按 Codex Goal 口径统计预算 token：未缓存输入 token + 输出 token。
// runtime 协议已将 cache creation/read 从 InputTokens 中独立拆出，不能再次扣减。
func (u GoalUsage) BudgetTokens() int64 {
	if u.BudgetTotalKnown || u.BudgetTotalTokens > 0 {
		return max(u.BudgetTotalTokens, 0)
	}
	if u.hasTokenBreakdown() {
		return max(u.InputTokens, 0) + max(u.OutputTokens, 0)
	}
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return 0
}

// ActualTokens 返回 runtime/provider 实际处理的 token 总量。
// 新记录优先使用 provider total；旧记录只能从 breakdown 做保守估算。
func (u GoalUsage) ActualTokens() int64 {
	if u.ActualTotalKnown || u.ActualTotalTokens > 0 {
		return max(u.ActualTotalTokens, 0)
	}
	if u.hasTokenBreakdown() {
		return max(u.InputTokens, 0) +
			max(u.CacheCreationInputTokens, 0) +
			max(u.CacheReadInputTokens, 0) +
			max(max(u.OutputTokens, 0), max(u.ReasoningTokens, 0))
	}
	return max(u.TotalTokens, 0)
}

// ActualTokensAreEstimated 判断 actual_tokens 是否由历史 breakdown 回填。
func (u GoalUsage) ActualTokensAreEstimated() bool {
	return u.ActualTokens() > 0 &&
		(u.ActualTokensEstimated || (!u.ActualTotalKnown && u.ActualTotalTokens <= 0))
}

// NormalizeTotals 同步显式 actual/budget 总量和旧 total_tokens 兼容别名。
func (u GoalUsage) NormalizeTotals() GoalUsage {
	explicitActual := u.ActualTotalKnown || u.ActualTotalTokens > 0
	budgetTokens := u.BudgetTokens()
	actualTokens := u.ActualTokens()
	u.InputTokens = max(u.InputTokens, 0)
	u.OutputTokens = max(u.OutputTokens, 0)
	u.CacheCreationInputTokens = max(u.CacheCreationInputTokens, 0)
	u.CacheReadInputTokens = max(u.CacheReadInputTokens, 0)
	u.ReasoningTokens = max(u.ReasoningTokens, 0)
	u.TotalTokens = budgetTokens
	u.BudgetTotalTokens = budgetTokens
	u.ActualTotalTokens = actualTokens
	u.ActualTokensEstimated = actualTokens > 0 && (u.ActualTokensEstimated || !explicitActual)
	u.BudgetTotalKnown = true
	u.ActualTotalKnown = true
	u.RuntimeSeconds = max(u.RuntimeSeconds, 0)
	return u
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
	left := u.NormalizeTotals()
	right := other.NormalizeTotals()
	budgetTokens := left.BudgetTotalTokens + right.BudgetTotalTokens
	return GoalUsage{
		InputTokens:              left.InputTokens + right.InputTokens,
		OutputTokens:             left.OutputTokens + right.OutputTokens,
		CacheCreationInputTokens: left.CacheCreationInputTokens + right.CacheCreationInputTokens,
		CacheReadInputTokens:     left.CacheReadInputTokens + right.CacheReadInputTokens,
		ReasoningTokens:          left.ReasoningTokens + right.ReasoningTokens,
		TotalTokens:              budgetTokens,
		BudgetTotalTokens:        budgetTokens,
		ActualTotalTokens:        left.ActualTotalTokens + right.ActualTotalTokens,
		ActualTokensEstimated:    left.ActualTokensEstimated || right.ActualTokensEstimated,
		RuntimeSeconds:           left.RuntimeSeconds + right.RuntimeSeconds,
		BudgetTotalKnown:         true,
		ActualTotalKnown:         true,
	}
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
	UsageFinalized     bool           `json:"usage_finalized"`
	UsageFinalizedAt   *time.Time     `json:"usage_finalized_at,omitempty"`
	LastError          string         `json:"last_error,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// ContinuationState derives the automatic continuation controller state from
// durable Goal fields. It is a read projection and is never persisted as a
// second source of truth.
func (g Goal) ContinuationState() GoalContinuationState {
	if NormalizeGoalStatus(g.Status) != GoalStatusActive {
		return GoalContinuationStateInactive
	}
	if strings.TrimSpace(g.LastError) != "" ||
		g.EmptyProgressCount >= GoalContinuationSuppressionThreshold {
		return GoalContinuationStateSuspended
	}
	if g.EmptyProgressCount > 0 {
		return GoalContinuationStateRecovering
	}
	return GoalContinuationStateReady
}

// MarshalJSON publishes the server-derived continuation state with every Goal
// REST and WebSocket projection without storing a denormalized state column.
func (g Goal) MarshalJSON() ([]byte, error) {
	type goalAlias Goal
	return json.Marshal(struct {
		*goalAlias
		ContinuationState GoalContinuationState `json:"continuation_state"`
	}{
		goalAlias:         (*goalAlias)(&g),
		ContinuationState: g.ContinuationState(),
	})
}

// GoalUsageReport 表示按 Goal ID 查询的稳定聚合 usage。
// UsageFinalized 为 true 时，Usage 不再接受迟到的 runtime 增量。
type GoalUsageReport struct {
	GoalID           string     `json:"goal_id"`
	SessionKey       string     `json:"session_key"`
	Status           GoalStatus `json:"status"`
	Usage            GoalUsage  `json:"usage"`
	TimeUsedSeconds  int64      `json:"time_used_seconds"`
	UsageFinalized   bool       `json:"usage_finalized"`
	UsageFinalizedAt *time.Time `json:"usage_finalized_at,omitempty"`
	GoalUpdatedAt    time.Time  `json:"goal_updated_at"`
}

// UsageReport 投影 Goal 的聚合 usage 查询结果。
func (g Goal) UsageReport() GoalUsageReport {
	return GoalUsageReport{
		GoalID:           g.ID,
		SessionKey:       g.SessionKey,
		Status:           NormalizeGoalStatus(g.Status),
		Usage:            g.Usage.NormalizeTotals(),
		TimeUsedSeconds:  max(g.TimeUsedSeconds, 0),
		UsageFinalized:   g.UsageFinalized,
		UsageFinalizedAt: g.UsageFinalizedAt,
		GoalUpdatedAt:    g.UpdatedAt,
	}
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

// ExplicitGoalReservedExecutionID derives the one Execution identity owned by
// an explicit create_goal command. The command is persisted before a WorkGraph
// exists, so the same reservation survives proposal retries and process restarts.
func ExplicitGoalReservedExecutionID(commandID string) string {
	commandID = strings.TrimSpace(commandID)
	if commandID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("explicit_goal_execution\x00" + commandID))
	return "execution_" + hex.EncodeToString(sum[:12])
}

// GoalReservedExecutionID returns the persisted Goal -> Execution reservation.
// Explicit Goals created before create_goal stored execution_id recover the same
// identity from their server-owned command instead of minting a new state chain.
func GoalReservedExecutionID(goal Goal) string {
	if GoalExecutionMode(GoalMetadataString(goal.Metadata, GoalMetadataExecutionMode)) ==
		GoalExecutionModeGoalOnly {
		return ""
	}
	if executionID := GoalMetadataString(goal.Metadata, GoalMetadataExecutionID); executionID != "" {
		return executionID
	}
	if GoalActivationOrigin(GoalMetadataString(goal.Metadata, GoalMetadataActivationOrigin)) !=
		GoalActivationOriginUserExplicit ||
		GoalActivationReason(GoalMetadataString(goal.Metadata, GoalMetadataActivationReason)) !=
			GoalActivationReasonPersistenceRequested {
		return ""
	}
	return ExplicitGoalReservedExecutionID(GoalMetadataString(
		goal.Metadata,
		GoalMetadataExplicitCommand,
	))
}

// GoalExecutionBindingStateFromGoal reads the persisted server-owned phase.
// Missing phase is a standalone/legacy record; malformed or resolver-only
// values fail closed as conflict.
func GoalExecutionBindingStateFromGoal(goal Goal) GoalExecutionBindingState {
	state := GoalExecutionBindingState(GoalMetadataString(
		goal.Metadata,
		GoalMetadataExecutionBindingState,
	))
	switch state {
	case GoalExecutionBindingStateReserved,
		GoalExecutionBindingStatePending,
		GoalExecutionBindingStateConfirmed:
		return state
	case "":
		return GoalExecutionBindingStateStandalone
	default:
		return GoalExecutionBindingStateConflict
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

// GoalMetadataInt64 从 Goal metadata 中读取 JSON 兼容的整数值。
func GoalMetadataInt64(metadata map[string]any, key string) int64 {
	value, ok := metadata[strings.TrimSpace(key)]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

// ObjectiveRevision 返回只随 objective 变化的 revision；旧数据从 1 起算。
func (g Goal) ObjectiveRevision() int64 {
	revision := GoalMetadataInt64(g.Metadata, GoalMetadataObjectiveRevision)
	if revision > 0 {
		return revision
	}
	return 1
}

// RemainingTokens 返回剩余 token 预算；没有预算时返回 nil。
func (g Goal) RemainingTokens() *int64 {
	if g.TokenBudget == nil || *g.TokenBudget <= 0 {
		return nil
	}
	remaining := *g.TokenBudget - g.Usage.BudgetTokens()
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
	ExecutionID    string            `json:"execution_id,omitempty"`
	RoundID        string            `json:"round_id"`
	Prompt         string            `json:"prompt"`
	HiddenFromUser bool              `json:"hidden_from_user"`
	Synthetic      bool              `json:"synthetic"`
	Purpose        string            `json:"purpose"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// GoalCommandOptions 是 UI set_goal 与 `/goal` 共用的可选控制参数。
// Room lead 始终来自服务端验证过的 target_agent_ids，不放入 metadata。
type GoalCommandOptions struct {
	TokenBudget     *int64         `json:"token_budget,omitempty"`
	ReplaceExisting *bool          `json:"replace_existing,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// GoalCommandRequest 是 host Goal command 进入 DM/Room 领域的稳定请求。
// CommandContent 只用于持久化用户实际执行的控制记录；Objective 才是 Goal 正文。
type GoalCommandRequest struct {
	SessionKey      string
	AgentID         string
	Objective       string
	CommandContent  string
	RoundID         string
	UserMessageID   string
	ClientRequestID string
	ClientMessageID string
	TargetAgentIDs  []string
	Options         GoalCommandOptions
}

// GoalCommandResult 返回 Goal 真相与控制记录是否已经 durable。
type GoalCommandResult struct {
	Goal                 Goal
	UserMessageCommitted bool
}

// CreateGoalRequest 表示创建 Goal 的请求。
type CreateGoalRequest struct {
	SessionKey      string         `json:"session_key"`
	Objective       string         `json:"objective"`
	TokenBudget     *int64         `json:"token_budget,omitempty"`
	ReplaceExisting bool           `json:"replace_existing,omitempty"`
	RoomLeadAgentID string         `json:"room_lead_agent_id,omitempty"`
	CreatedBy       string         `json:"created_by,omitempty"`
	RoundID         string         `json:"round_id,omitempty"`
	OwnerUserID     string         `json:"owner_user_id,omitempty"`
	AgentID         string         `json:"-"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// UpdateGoalRequest 表示更新 Goal 的请求。
type UpdateGoalRequest struct {
	Objective   *string        `json:"objective,omitempty"`
	TokenBudget OptionalInt64  `json:"token_budget,omitempty"`
	OwnerUserID string         `json:"owner_user_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	// Room ownership fields are server-derived command context.
	// They are never accepted from HTTP or WebSocket JSON.
	RoomLeadAgentID   string `json:"-"`
	RoomLeadAgentName string `json:"-"`
}

// RetargetGoalRequest 表示模型基于用户明确纠正重定向当前 active Goal。
type RetargetGoalRequest struct {
	Objective                 string `json:"objective"`
	RoundID                   string `json:"round_id,omitempty"`
	AgentID                   string `json:"-"`
	ExpectedGoalID            string `json:"-"`
	ExpectedObjectiveRevision int64  `json:"-"`
}

// CompleteGoalRequest 表示完成 Goal 的请求。
type CompleteGoalRequest struct {
	Summary                   string `json:"summary,omitempty"`
	RoundID                   string `json:"round_id,omitempty"`
	AgentID                   string `json:"-"`
	ExpectedObjectiveRevision int64  `json:"-"`
}

// BlockGoalRequest 表示阻塞 Goal 的请求。
type BlockGoalRequest struct {
	Reason                    string `json:"reason"`
	NeededInput               string `json:"needed_input,omitempty"`
	RoundID                   string `json:"round_id,omitempty"`
	AgentID                   string `json:"-"`
	ExpectedObjectiveRevision int64  `json:"-"`
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

// IsGoalUsageFinalizableStatus 判断 Goal 是否已进入可结算最终 usage 的终态。
func IsGoalUsageFinalizableStatus(status GoalStatus) bool {
	switch NormalizeGoalStatus(status) {
	case GoalStatusComplete:
		return true
	default:
		return false
	}
}
