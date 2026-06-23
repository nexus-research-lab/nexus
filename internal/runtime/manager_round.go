package runtime

import (
	"context"
	"maps"
	"slices"
	"strings"
)

// StartRound 注册运行中的 round，并记录其取消函数。
func (m *Manager) StartRound(sessionKey string, roundID string, cancel context.CancelFunc) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	state.RunningRounds[roundID] = struct{}{}
	m.touchStateLocked(state)
	delete(state.Interruptions, roundID)
	if cancel != nil {
		state.RoundCancels[roundID] = cancel
	}
	if _, exists := state.RoundDone[roundID]; !exists {
		state.RoundDone[roundID] = make(chan struct{})
	}
}

// MarkRoundFinished 把 round 从运行态中移除。
func (m *Manager) MarkRoundFinished(sessionKey string, roundID string) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		return
	}
	delete(state.RunningRounds, roundID)
	delete(state.RoundCancels, roundID)
	delete(state.Interruptions, roundID)
	delete(state.GoalAccountingFlushers, roundID)
	delete(state.GoalAccountingClearers, roundID)
	delete(state.GoalAccountingActivators, roundID)
	m.touchStateLocked(state)
	if len(state.RunningRounds) == 0 {
		state.GuidedInputs = nil
	}
	if done, ok := state.RoundDone[roundID]; ok {
		close(done)
		delete(state.RoundDone, roundID)
	}
}

// GetRunningRoundIDs 返回当前 session 的运行中轮次。
func (m *Manager) GetRunningRoundIDs(sessionKey string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.sessions[sessionKey]
	if !ok || len(state.RunningRounds) == 0 {
		return []string{}
	}
	return slices.Sorted(maps.Keys(state.RunningRounds))
}

// CountRunningRounds 统计指定 Agent 当前活跃 round 数量。
func (m *Manager) CountRunningRounds(agentID string) int {
	if agentID == "" {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for sessionKey, state := range m.sessions {
		if len(state.RunningRounds) == 0 {
			continue
		}
		if !strings.HasPrefix(sessionKey, "agent:"+agentID+":") {
			continue
		}
		total += len(state.RunningRounds)
	}
	return total
}
