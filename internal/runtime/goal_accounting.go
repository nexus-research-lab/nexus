package runtime

import (
	"context"
	"maps"
	"slices"
	"strings"
)

// GoalAccountingFlush 由正在运行的 round 提供，用于外部 Goal 状态变化前结算当前进度。
type GoalAccountingFlush func(context.Context) error

// GoalAccountingClear 由正在运行的 round 提供，用于 Goal 停止后关闭后续计量。
type GoalAccountingClear func()

// GoalAccountingActivate 由正在运行的 round 提供，用于 Goal 恢复 active 后重置计量基线。
type GoalAccountingActivate func(context.Context) error

// RegisterGoalAccountingFlush 注册或移除运行中 round 的 Goal accounting flush 回调。
func (m *Manager) RegisterGoalAccountingFlush(sessionKey string, roundID string, flush GoalAccountingFlush) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if flush == nil {
		delete(state.GoalAccountingFlushers, roundID)
		return
	}
	state.GoalAccountingFlushers[roundID] = flush
}

// RegisterGoalAccountingClear 注册或移除运行中 round 的 Goal accounting clear 回调。
func (m *Manager) RegisterGoalAccountingClear(sessionKey string, roundID string, clear GoalAccountingClear) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if clear == nil {
		delete(state.GoalAccountingClearers, roundID)
		return
	}
	state.GoalAccountingClearers[roundID] = clear
}

// RegisterGoalAccountingActivate 注册或移除运行中 round 的 Goal accounting active 回调。
func (m *Manager) RegisterGoalAccountingActivate(sessionKey string, roundID string, activate GoalAccountingActivate) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.GoalAccountingActivators == nil {
		state.GoalAccountingActivators = make(map[string]GoalAccountingActivate)
	}
	if activate == nil {
		delete(state.GoalAccountingActivators, roundID)
		return
	}
	state.GoalAccountingActivators[roundID] = activate
}

// FlushGoalAccounting 要求指定 session 的运行中 round 结算当前 Goal progress。
func (m *Manager) FlushGoalAccounting(ctx context.Context, sessionKey string) ([]string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil, nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingFlushers) == 0 {
		m.mu.RUnlock()
		return nil, nil
	}
	roundIDs := slices.Sorted(maps.Keys(state.GoalAccountingFlushers))
	flushers := make([]GoalAccountingFlush, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		flushers = append(flushers, state.GoalAccountingFlushers[roundID])
	}
	m.mu.RUnlock()

	var firstErr error
	flushed := make([]string, 0, len(roundIDs))
	for index, flush := range flushers {
		if flush == nil {
			continue
		}
		if err := flush(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		flushed = append(flushed, roundIDs[index])
	}
	return flushed, firstErr
}

// ClearGoalAccounting 要求指定 session 的运行中 round 停止把后续 usage 归属到当前 Goal。
func (m *Manager) ClearGoalAccounting(sessionKey string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingClearers) == 0 {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := slices.Sorted(maps.Keys(state.GoalAccountingClearers))
	clearers := make([]GoalAccountingClear, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		clearers = append(clearers, state.GoalAccountingClearers[roundID])
	}
	m.mu.RUnlock()

	cleared := make([]string, 0, len(roundIDs))
	for index, clear := range clearers {
		if clear == nil {
			continue
		}
		clear()
		cleared = append(cleared, roundIDs[index])
	}
	return cleared
}

// ActivateGoalAccounting 要求指定 session 的运行中 round 从当前快照开始归属 Goal usage。
func (m *Manager) ActivateGoalAccounting(ctx context.Context, sessionKey string) ([]string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil, nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingActivators) == 0 {
		m.mu.RUnlock()
		return nil, nil
	}
	roundIDs := slices.Sorted(maps.Keys(state.GoalAccountingActivators))
	activators := make([]GoalAccountingActivate, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		activators = append(activators, state.GoalAccountingActivators[roundID])
	}
	m.mu.RUnlock()

	var firstErr error
	activated := make([]string, 0, len(roundIDs))
	for index, activate := range activators {
		if activate == nil {
			continue
		}
		if err := activate(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		activated = append(activated, roundIDs[index])
	}
	return activated, firstErr
}
