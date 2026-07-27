// INPUT: runtime 来源的 child 累计 token、Room parent 终态 usage 与 durable scope 身份。
// OUTPUT: 跨进程去重、scope 绑定/回补及可与 Goal usage 原子提交的来源记录协议。
// POS: child/Room parent usage 在 runtime/service/storage 之间的内部契约。
package protocol

import "time"

const GoalUsageSourceKindNXSTask = "nxs_task"

// GoalUsageSourceSnapshot 描述一个 runtime source 的单调累计 actual token。
// GoalID 为空时，GoalSessionKey + ScopeRoundID 仍标识可被后续 Goal 原子认领的
// durable scope；若该 scope 已绑定且 source 不带 external activation 的
// baseline-unavailable 标记，新增 delta 会直接归入绑定 Goal。
type GoalUsageSourceSnapshot struct {
	OwnerUserID            string
	RuntimeSessionKey      string
	SourceKind             string
	SourceID               string
	CumulativeActualTokens int64
	// EvidenceRequired 表示该 source 是当前 scope 内真实启动过的 child task，
	// 即使当前尚无正 token，也必须留下 durable lifecycle evidence。
	EvidenceRequired bool
	// Terminal 描述 child 生命周期终态；TokenUsageObserved 表示当前 snapshot
	// 携带正 provider total。只有二者同时为 true 才构成 terminal token evidence；
	// progress 正数不能替代终态结算。当前 nxs 的 total_tokens=0 是缺失占位。
	Terminal           bool
	TokenUsageObserved bool
	GoalID             string
	GoalSessionKey     string
	RoundID            string
	ScopeRoundID       string
	EventID            string
	ObservedAt         time.Time
}

// GoalUsageSourceRoundClaim 建立 scope 到 Goal 的 durable 绑定，并把该 scope 下
// 跨 runtime session 暂存的 source 增量原子归入 Goal。
type GoalUsageSourceRoundClaim struct {
	OwnerUserID string
	// RuntimeSessionKey 仅为旧调用方与审计 payload 保留；认领范围不再按它过滤。
	RuntimeSessionKey string
	SourceKind        string
	RoundID           string
	ScopeRoundID      string
	GoalID            string
	GoalSessionKey    string
	EventID           string
	ClaimedAt         time.Time
}

// GoalUsageScopeBinding 是 owner/session/source kind/scope 到 Goal 的持久绑定。
// UsageEventID 是绑定建立时首次 pending 回补所使用的审计事件身份。
type GoalUsageScopeBinding struct {
	OwnerUserID    string
	GoalSessionKey string
	SourceKind     string
	ScopeRoundID   string
	GoalID         string
	BoundAt        time.Time
	UsageEventID   string
}

// GoalUsageScopeCreateResult 是 Goal、created 事件、scope 绑定和首次 pending
// 回补在同一事务提交后的结果。
type GoalUsageScopeCreateResult struct {
	Goal                  *Goal
	UsageEvent            *GoalEvent
	AttributedDelta       int64
	AttributedUsage       GoalUsage
	TokenUsageUnavailable bool
}

// GoalUsageSourceResult 是 checkpoint 与 Goal usage 原子提交后的结果。
type GoalUsageSourceResult struct {
	ObservedDelta         int64
	AttributedDelta       int64
	AttributedUsage       GoalUsage
	TokenUsageUnavailable bool
	Goal                  *Goal
	Event                 *GoalEvent
}

// GoalUsageParentSnapshot 是一个 Room parent slot 的终态 usage 证据。
//
// SourceRoundID 在同一 owner/session/root scope 下幂等标识一个 slot。GoalID
// 通常留空并由 durable scope 解析；它只为已经在内存中明确绑定 Goal 的终态
// 快照保留。TokenUsageObserved 区分 provider 明确返回 total=0 与完全缺少 usage。
type GoalUsageParentSnapshot struct {
	OwnerUserID        string
	GoalSessionKey     string
	ScopeRoundID       string
	SourceRoundID      string
	GoalID             string
	Usage              GoalUsage
	TokenUsageObserved bool
	EventID            string
	ObservedAt         time.Time
}

// GoalUsageParentResult 是 Room parent 终态证据持久化及可选即时归属的结果。
type GoalUsageParentResult struct {
	AttributedUsage       GoalUsage
	TokenUsageUnavailable bool
	Goal                  *Goal
	Event                 *GoalEvent
}

// GoalUsageScopeBindResult 描述 external activation 从当前时刻绑定 scope 时，
// 被明确排除在新 Goal 之前的 durable pending 数量。
type GoalUsageScopeBindResult struct {
	DiscardedChildPending  int64
	DiscardedChildEvidence int64
	DiscardedParentPending int64
}
