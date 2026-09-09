// INPUT: 子任务累计用量、终态证据及原始观察时间。
// OUTPUT: 可单调合并、按已落库证据确认的待结算观察值。
// POS: Goal 子任务结算规则；宿主持有锁、pending 容器与重试任务。
package goal

import "time"

// SubagentUsageObservation 保留尚未确认持久化的子任务证据。
// 重试沿用 ObservedAt，避免把旧观察移动到外部 Goal 换绑之后。
type SubagentUsageObservation struct {
	CumulativeTotal            int64
	Terminal                   bool
	TerminalTokenUsageObserved bool
	ObservedAt                 time.Time
}

// Merge 只推进累计值和终态；只有新增累计值或首次终态改变观察时间。
func (current SubagentUsageObservation) Merge(observation SubagentUsageObservation) SubagentUsageObservation {
	if observation.CumulativeTotal > current.CumulativeTotal {
		current.CumulativeTotal = observation.CumulativeTotal
		current.ObservedAt = observation.ObservedAt
	}
	if observation.Terminal && !current.Terminal {
		current.ObservedAt = observation.ObservedAt
	}
	if current.ObservedAt.IsZero() {
		current.ObservedAt = observation.ObservedAt
	}
	current.Terminal = current.Terminal || observation.Terminal
	current.TerminalTokenUsageObserved = current.TerminalTokenUsageObserved || observation.TerminalTokenUsageObserved
	return current
}

// CoveredBy 防止旧 checkpoint 的成功回执清除后来到达的累计值或终态证据。
func (pending SubagentUsageObservation) CoveredBy(settled SubagentUsageObservation) bool {
	return pending.CumulativeTotal <= settled.CumulativeTotal &&
		(!pending.Terminal || settled.Terminal) &&
		(!pending.TerminalTokenUsageObserved || settled.TerminalTokenUsageObserved)
}
