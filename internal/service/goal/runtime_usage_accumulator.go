// INPUT: runtime 的逐 turn/cumulative token 快照与 Goal accounting 激活边界。
// OUTPUT: 按 turn 去重、由最终 result 校准的 actual/budget token 与耗时增量。
// POS: runtime usage 到 Goal 持久化增量之间的归一化层。
package goal

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RuntimeUsageSnapshot 表示一次 runtime usage 快照。
type RuntimeUsageSnapshot struct {
	Usage          protocol.GoalUsage
	ElapsedSeconds int64
	// TokenUsageObserved 区分“runtime 明确报告 0 token”和“终态完全没有
	// provider/assistant usage”。只有前者可以建立 authoritative finalization fence。
	TokenUsageObserved bool
	// TurnID 标识逐 provider turn 的 assistant message；同 ID 快照只补差。
	TurnID string
	// Cumulative 表示 Usage 是整个 round 的累计 result，而不是单个 turn。
	Cumulative bool
	// Terminal 表示这是 round 的最终可用快照；只有终态才会把 token
	// breakdown/预算/actual 落库，避免累计真值无法向下校准中间快照。
	Terminal bool
	// SettlementBoundary 表示外部 mutation 即将换绑或停止当前 Goal。
	// 此时必须结算已经观察到的 actual；否则随后的 Reset/Close 会丢失该前缀。
	SettlementBoundary bool
}

// RuntimeUsageAccumulator 把逐 turn assistant usage 与累计 result 统一成 Goal 增量。
type RuntimeUsageAccumulator struct {
	active                               bool
	finalizing                           bool
	closed                               bool
	activated                            bool
	turns                                map[string]protocol.GoalUsage
	roundUsage                           protocol.GoalUsage
	excludedUsage                        protocol.GoalUsage
	emittedUsage                         protocol.GoalUsage
	roundElapsed                         int64
	excludedElapsed                      int64
	emittedElapsed                       int64
	tokenUsageObservationVersion         uint64
	excludedTokenUsageObservationVersion uint64
}

// NewRuntimeUsageAccumulator 创建 Goal usage 增量结算器。
func NewRuntimeUsageAccumulator(active bool) *RuntimeUsageAccumulator {
	return &RuntimeUsageAccumulator{
		active:    active,
		activated: active,
		turns:     make(map[string]protocol.GoalUsage),
	}
}

// Active 返回当前 round 是否正在把后续 usage 归属到 Goal。
func (a *RuntimeUsageAccumulator) Active() bool {
	return a != nil && a.active && !a.closed
}

// TokenUsageObserved 返回当前 Goal 归属区间是否至少收到过一个 provider 或
// assistant token usage；显式 0 算已观察，Reset 前的外部前缀不算。
func (a *RuntimeUsageAccumulator) TokenUsageObserved() bool {
	return a != nil &&
		a.tokenUsageObservationVersion > a.excludedTokenUsageObservationVersion
}

// EligibleForUnboundTerminal 仅允许从未绑定过 Goal 的 pristine accumulator
// 建立 Room unbound terminal ledger；曾激活、Reset 或 Close 后均不可重新开放。
func (a *RuntimeUsageAccumulator) EligibleForUnboundTerminal() bool {
	return a != nil && !a.active && !a.finalizing && !a.closed && !a.activated
}

// Reset 静默吸收当前快照，以当前 round 前缀为基线开始记录后续 usage。
// 该入口用于外部创建/恢复 Goal，激活前已经完成的工作不归入 Goal。
func (a *RuntimeUsageAccumulator) Reset(snapshot RuntimeUsageSnapshot) {
	if a == nil {
		return
	}
	a.observe(snapshot)
	a.active = true
	a.activated = true
	a.finalizing = false
	a.closed = false
	a.excludedUsage = a.roundUsage
	a.emittedUsage = protocol.GoalUsage{}
	a.excludedElapsed = a.roundElapsed
	a.emittedElapsed = 0
	a.excludedTokenUsageObservationVersion = a.tokenUsageObservationVersion
}

// ActivateFromRoundStart 从 round 起点激活 accounting，并返回此前已观察到的 backlog。
// 该入口用于模型在执行过程中创建 Goal：创建前同一 round 的工作也属于该 Goal。
func (a *RuntimeUsageAccumulator) ActivateFromRoundStart() (protocol.GoalUsage, bool) {
	delta, ok := a.PrepareActivationFromRoundStart()
	if ok {
		a.CommitDelta(delta)
	}
	return delta, ok
}

// PrepareActivationFromRoundStart 激活 round-start accounting，但不推进 emission watermark。
// 调用方只有在持久化成功后才能 CommitDelta；失败时后续快照会重试完整增量。
func (a *RuntimeUsageAccumulator) PrepareActivationFromRoundStart() (protocol.GoalUsage, bool) {
	if a == nil {
		return protocol.GoalUsage{}, false
	}
	if a.Active() || a.finalizing || a.closed || a.activated {
		return protocol.GoalUsage{}, false
	}
	a.active = true
	a.activated = true
	a.finalizing = false
	a.closed = false
	a.excludedUsage = protocol.GoalUsage{}
	a.emittedUsage = protocol.GoalUsage{}
	a.excludedElapsed = 0
	a.emittedElapsed = 0
	a.excludedTokenUsageObservationVersion = 0
	return a.pendingDelta(false)
}

// BeginFinalizing 保留当前 round 的 Goal 绑定，直到 terminal usage 完成对账。
func (a *RuntimeUsageAccumulator) BeginFinalizing() {
	if a == nil || a.closed {
		return
	}
	a.active = true
	a.activated = true
	a.finalizing = true
}

// Close 停止把当前 round 的后续 usage 归属到 Goal。
func (a *RuntimeUsageAccumulator) Close() {
	if a == nil {
		return
	}
	a.finalizing = false
	a.closed = true
}

// Delta 吸收逐 turn 或累计快照，返回当前 Goal 区间尚未落库的增量。
// inactive/closed 状态仍会观察快照，以便模型创建 Goal 时可从 round 起点结算。
func (a *RuntimeUsageAccumulator) Delta(snapshot RuntimeUsageSnapshot) (protocol.GoalUsage, bool) {
	delta, ok := a.PrepareDelta(snapshot)
	if ok {
		a.CommitDelta(delta)
	}
	return delta, ok
}

// PrepareDelta 吸收快照并返回尚未持久化的增量，不推进 emission watermark。
func (a *RuntimeUsageAccumulator) PrepareDelta(snapshot RuntimeUsageSnapshot) (protocol.GoalUsage, bool) {
	if a == nil {
		return protocol.GoalUsage{}, false
	}
	a.observe(snapshot)
	if !a.Active() {
		return protocol.GoalUsage{}, false
	}
	return a.pendingDelta(snapshot.Terminal || snapshot.SettlementBoundary)
}

// CommitDelta 仅在调用方确认增量已持久化后推进 emission watermark。
func (a *RuntimeUsageAccumulator) CommitDelta(delta protocol.GoalUsage) {
	if a == nil || isGoalUsageZero(delta) {
		return
	}
	a.emittedUsage = a.emittedUsage.Add(delta)
	a.emittedElapsed += max(delta.RuntimeSeconds, 0)
}

func (a *RuntimeUsageAccumulator) observe(snapshot RuntimeUsageSnapshot) {
	currentUsage := snapshot.Usage.NormalizeTotals()
	a.roundElapsed = max(a.roundElapsed, positiveInt64(snapshot.ElapsedSeconds))
	if snapshot.TokenUsageObserved {
		a.tokenUsageObservationVersion++
	}
	if snapshot.Cumulative {
		// result 累计 usage 是本轮 token 真相源。中间逐 turn 快照可能更大，
		// token 尚未落库，因此可以在 terminal 时连同 components/budget 一起
		// 向下校准。只有 provider total 的 result 则保留逐 turn breakdown/budget，
		// 仅替换 actual provenance；耗时在独立 watermark 中保持单调。
		if goalUsageHasBreakdownOrBudget(currentUsage) {
			a.roundUsage = currentUsage
		} else {
			a.roundUsage.ActualTotalTokens = currentUsage.ActualTokens()
			a.roundUsage.ActualTokensEstimated = currentUsage.ActualTokensAreEstimated()
			a.roundUsage.ActualTotalKnown = true
		}
		return
	}
	if strings.TrimSpace(snapshot.TurnID) == "" {
		a.roundUsage = maxGoalUsage(a.roundUsage, currentUsage)
		return
	}
	turnID := strings.TrimSpace(snapshot.TurnID)
	previous := a.turns[turnID]
	currentUsage = maxGoalUsage(previous, currentUsage)
	increment := subtractGoalUsage(currentUsage, previous)
	if !isGoalUsageZero(increment) {
		a.roundUsage = a.roundUsage.Add(increment)
	}
	a.turns[turnID] = currentUsage
}

func goalUsageHasBreakdownOrBudget(usage protocol.GoalUsage) bool {
	return usage.InputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 ||
		usage.CacheReadInputTokens > 0 ||
		usage.ReasoningTokens > 0 ||
		usage.BudgetTokens() > 0
}

func (a *RuntimeUsageAccumulator) pendingDelta(allowTokens bool) (protocol.GoalUsage, bool) {
	target := subtractGoalUsage(a.roundUsage, a.excludedUsage)
	delta := subtractGoalUsage(target, a.emittedUsage)
	targetElapsed := saturatingSub(a.roundElapsed, a.excludedElapsed)
	delta.RuntimeSeconds = saturatingSub(targetElapsed, a.emittedElapsed)
	if !allowTokens {
		// Goal 聚合只增不减，所有中间 token 字段都必须留在内存中等待
		// terminal cumulative 或显式 settlement boundary。只允许耗时提前落库，
		// 否则 200-token 中间快照无法被 180-token 终态真值校准。
		delta = protocol.GoalUsage{RuntimeSeconds: delta.RuntimeSeconds}
	}
	return delta, !isGoalUsageZero(delta)
}

func subtractGoalUsage(current protocol.GoalUsage, previous protocol.GoalUsage) protocol.GoalUsage {
	current = current.NormalizeTotals()
	previous = previous.NormalizeTotals()
	actualTokens := saturatingSub(current.ActualTokens(), previous.ActualTokens())
	budgetTokens := saturatingSub(current.BudgetTokens(), previous.BudgetTokens())
	return (protocol.GoalUsage{
		InputTokens:              saturatingSub(current.InputTokens, previous.InputTokens),
		OutputTokens:             saturatingSub(current.OutputTokens, previous.OutputTokens),
		CacheCreationInputTokens: saturatingSub(current.CacheCreationInputTokens, previous.CacheCreationInputTokens),
		CacheReadInputTokens:     saturatingSub(current.CacheReadInputTokens, previous.CacheReadInputTokens),
		ReasoningTokens:          saturatingSub(current.ReasoningTokens, previous.ReasoningTokens),
		TotalTokens:              budgetTokens,
		BudgetTotalTokens:        budgetTokens,
		ActualTotalTokens:        actualTokens,
		ActualTokensEstimated: actualTokens > 0 &&
			(current.ActualTokensAreEstimated() || previous.ActualTokensAreEstimated()),
		BudgetTotalKnown: true,
		ActualTotalKnown: true,
	}).NormalizeTotals()
}

func isGoalUsageZero(usage protocol.GoalUsage) bool {
	return usage.BudgetTokens() == 0 &&
		usage.ActualTokens() == 0 &&
		usage.RuntimeSeconds == 0
}

func maxGoalUsage(left protocol.GoalUsage, right protocol.GoalUsage) protocol.GoalUsage {
	return protocol.GoalUsage{
		InputTokens:              max(left.InputTokens, right.InputTokens),
		OutputTokens:             max(left.OutputTokens, right.OutputTokens),
		CacheCreationInputTokens: max(left.CacheCreationInputTokens, right.CacheCreationInputTokens),
		CacheReadInputTokens:     max(left.CacheReadInputTokens, right.CacheReadInputTokens),
		ReasoningTokens:          max(left.ReasoningTokens, right.ReasoningTokens),
		TotalTokens:              max(left.TotalTokens, right.TotalTokens),
		BudgetTotalTokens:        max(left.BudgetTotalTokens, right.BudgetTotalTokens),
		ActualTotalTokens:        max(left.ActualTotalTokens, right.ActualTotalTokens),
		ActualTokensEstimated:    left.ActualTokensEstimated || right.ActualTokensEstimated,
		RuntimeSeconds:           max(left.RuntimeSeconds, right.RuntimeSeconds),
		BudgetTotalKnown:         left.BudgetTotalKnown || right.BudgetTotalKnown,
		ActualTotalKnown:         left.ActualTotalKnown || right.ActualTotalKnown,
	}
}

func saturatingSub(current int64, previous int64) int64 {
	return max(current-previous, 0)
}

func positiveInt64(value int64) int64 {
	return max(value, 0)
}
