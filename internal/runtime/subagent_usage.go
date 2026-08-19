// INPUT: runtime session、task ID 与 nxs 上报的累计 child task token。
// OUTPUT: 相对同一 task 历史高水位的非负 token 增量。
// POS: 跨 round/idle drain 的后台子 Agent usage 去重状态。
package runtime

import "strings"

// ObserveSubagentUsage 把单调累计的 child task total 转换为跨 round 去重增量。
// nxs task follow-up 会复用 task ID，因此高水位跟随 runtime session 生命周期，
// 不能放在短生命周期的 round runner 中。
func (m *Manager) ObserveSubagentUsage(sessionKey string, taskID string, cumulativeTokens int64) int64 {
	if cumulativeTokens <= 0 {
		return 0
	}
	sessionKey = strings.TrimSpace(sessionKey)
	taskID = strings.TrimSpace(taskID)
	if sessionKey == "" || taskID == "" {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionKey + "\x00" + taskID
	previous := m.subagentUsageTotals[key]
	if cumulativeTokens <= previous {
		return 0
	}
	m.subagentUsageTotals[key] = cumulativeTokens
	state := m.ensureStateLocked(sessionKey)
	m.touchStateLocked(state)
	return cumulativeTokens - previous
}
