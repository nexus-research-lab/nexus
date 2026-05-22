package goal

import "github.com/nexus-research-lab/nexus/internal/protocol"

// RuntimeUsageSnapshot 表示一次 runtime 已累计 usage 快照。
type RuntimeUsageSnapshot struct {
	Usage          protocol.GoalUsage
	ElapsedSeconds int64
}

// RuntimeUsageAccumulator 把 runtime 的累计 usage 转成 Goal 可记录的增量。
type RuntimeUsageAccumulator struct {
	active             bool
	closed             bool
	lastUsage          protocol.GoalUsage
	lastElapsedSeconds int64
}

// NewRuntimeUsageAccumulator 创建 Goal usage 增量结算器。
func NewRuntimeUsageAccumulator(active bool) *RuntimeUsageAccumulator {
	return &RuntimeUsageAccumulator{active: active}
}

// Active 返回当前 round 是否正在把后续 usage 归属到 Goal。
func (a *RuntimeUsageAccumulator) Active() bool {
	return a != nil && a.active && !a.closed
}

// Reset 用当前快照作为新基线，并开始记录后续 usage。
func (a *RuntimeUsageAccumulator) Reset(snapshot RuntimeUsageSnapshot) {
	if a == nil {
		return
	}
	a.active = true
	a.closed = false
	a.lastUsage = normalizeSnapshotUsage(snapshot)
	a.lastElapsedSeconds = positiveInt64(snapshot.ElapsedSeconds)
}

// Close 停止把当前 round 的后续 usage 归属到 Goal。
func (a *RuntimeUsageAccumulator) Close() {
	if a == nil {
		return
	}
	a.closed = true
}

// Delta 返回当前累计快照相对上次基线的增量，并推进基线。
func (a *RuntimeUsageAccumulator) Delta(snapshot RuntimeUsageSnapshot) (protocol.GoalUsage, bool) {
	if !a.Active() {
		return protocol.GoalUsage{}, false
	}
	currentUsage := normalizeSnapshotUsage(snapshot)
	currentElapsed := positiveInt64(snapshot.ElapsedSeconds)
	delta := protocol.GoalUsage{
		InputTokens:              saturatingSub(currentUsage.InputTokens, a.lastUsage.InputTokens),
		OutputTokens:             saturatingSub(currentUsage.OutputTokens, a.lastUsage.OutputTokens),
		CacheCreationInputTokens: saturatingSub(currentUsage.CacheCreationInputTokens, a.lastUsage.CacheCreationInputTokens),
		CacheReadInputTokens:     saturatingSub(currentUsage.CacheReadInputTokens, a.lastUsage.CacheReadInputTokens),
		ReasoningTokens:          saturatingSub(currentUsage.ReasoningTokens, a.lastUsage.ReasoningTokens),
		RuntimeSeconds:           saturatingSub(currentElapsed, a.lastElapsedSeconds),
	}
	delta.TotalTokens = delta.BudgetTokens()
	a.lastUsage = maxGoalUsage(a.lastUsage, currentUsage)
	a.lastElapsedSeconds = maxInt64(a.lastElapsedSeconds, currentElapsed)
	return delta, !isGoalUsageZero(delta)
}

func normalizeSnapshotUsage(snapshot RuntimeUsageSnapshot) protocol.GoalUsage {
	usage := snapshot.Usage
	usage.InputTokens = positiveInt64(usage.InputTokens)
	usage.OutputTokens = positiveInt64(usage.OutputTokens)
	usage.CacheCreationInputTokens = positiveInt64(usage.CacheCreationInputTokens)
	usage.CacheReadInputTokens = positiveInt64(usage.CacheReadInputTokens)
	usage.ReasoningTokens = positiveInt64(usage.ReasoningTokens)
	usage.TotalTokens = positiveInt64(usage.TotalTokens)
	usage.RuntimeSeconds = positiveInt64(usage.RuntimeSeconds)
	return usage
}

func isGoalUsageZero(usage protocol.GoalUsage) bool {
	return usage.InputTokens == 0 &&
		usage.OutputTokens == 0 &&
		usage.CacheCreationInputTokens == 0 &&
		usage.CacheReadInputTokens == 0 &&
		usage.ReasoningTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.RuntimeSeconds == 0
}

func maxGoalUsage(left protocol.GoalUsage, right protocol.GoalUsage) protocol.GoalUsage {
	return protocol.GoalUsage{
		InputTokens:              maxInt64(left.InputTokens, right.InputTokens),
		OutputTokens:             maxInt64(left.OutputTokens, right.OutputTokens),
		CacheCreationInputTokens: maxInt64(left.CacheCreationInputTokens, right.CacheCreationInputTokens),
		CacheReadInputTokens:     maxInt64(left.CacheReadInputTokens, right.CacheReadInputTokens),
		ReasoningTokens:          maxInt64(left.ReasoningTokens, right.ReasoningTokens),
		TotalTokens:              maxInt64(left.TotalTokens, right.TotalTokens),
		RuntimeSeconds:           maxInt64(left.RuntimeSeconds, right.RuntimeSeconds),
	}
}

func saturatingSub(current int64, previous int64) int64 {
	if current <= previous {
		return 0
	}
	return current - previous
}

func positiveInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
