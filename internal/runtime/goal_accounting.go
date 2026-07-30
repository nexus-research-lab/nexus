// INPUT: 运行中 round 的 Goal accounting 回调与 objective revision 指针。
// OUTPUT: session/round 级结算、清理、激活和 revision adoption。
// POS: runtime Manager 中 Goal 执行态的注册与并发协调入口。
package runtime

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
)

// GoalAccountingFlush 由正在运行的 round 提供，用于外部 Goal 状态变化前结算当前进度。
type GoalAccountingFlush func(context.Context) error

// GoalAccountingClear 由正在运行的 round 提供，用于 Goal 停止后关闭后续计量。
type GoalAccountingClear func()

// GoalAccountingFinalize 由正在运行的 round 提供，用于 complete Goal 保留固定
// 绑定直到 provider terminal 与 child drain 完成；返回当前 callback 是否真正接管。
type GoalAccountingFinalize func() bool

// GoalAccountingActivate 由正在运行的 round 提供，用于 Goal 恢复 active 后绑定
// 明确 Goal ID 并重置计量基线。
type GoalAccountingActivate func(context.Context, string) error

// GoalAccountingConsumed 判断 live runtime scope 是否已经消费过一个 Goal。
// 一旦返回 true，该 scope 在 round 结束前不能承载另一个 Goal。
type GoalAccountingConsumed func() bool

type goalAccountingGuard struct {
	scopeRoundID string
	consumed     GoalAccountingConsumed
}

// RegisterGoalObjectiveRevision 让运行中 round 的 MCP 与终态回调共享同一 objective revision。
func (m *Manager) RegisterGoalObjectiveRevision(sessionKey string, roundID string, revision *atomic.Int64) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.Closing {
		return
	}
	if state.GoalObjectiveRevisions == nil {
		state.GoalObjectiveRevisions = make(map[string]*atomic.Int64)
	}
	if revision == nil {
		delete(state.GoalObjectiveRevisions, roundID)
		return
	}
	state.GoalObjectiveRevisions[roundID] = revision
}

// AdoptGoalObjectiveRevision 在 steering 真正被 runtime 消费后推进运行中 round 的 revision fence。
func (m *Manager) AdoptGoalObjectiveRevision(sessionKey string, revision int64) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" || revision <= 0 {
		return nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalObjectiveRevisions) == 0 {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := slices.Sorted(maps.Keys(state.GoalObjectiveRevisions))
	revisions := make([]*atomic.Int64, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		revisions = append(revisions, state.GoalObjectiveRevisions[roundID])
	}
	m.mu.RUnlock()

	adopted := make([]string, 0, len(roundIDs))
	for index, state := range revisions {
		if state == nil {
			continue
		}
		for {
			current := state.Load()
			if revision <= current || state.CompareAndSwap(current, revision) {
				break
			}
		}
		adopted = append(adopted, roundIDs[index])
	}
	return adopted
}

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
	if state.Closing {
		return
	}
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
	if state.Closing {
		return
	}
	if clear == nil {
		delete(state.GoalAccountingClearers, roundID)
		return
	}
	state.GoalAccountingClearers[roundID] = clear
}

// RegisterGoalAccountingFinalize 注册或移除运行中 round 的 Goal terminal 对账回调。
func (m *Manager) RegisterGoalAccountingFinalize(
	sessionKey string,
	roundID string,
	finalize GoalAccountingFinalize,
) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.GoalAccountingFinalizers == nil {
		state.GoalAccountingFinalizers = make(map[string]GoalAccountingFinalize)
	}
	if finalize == nil {
		delete(state.GoalAccountingFinalizers, roundID)
		return
	}
	state.GoalAccountingFinalizers[roundID] = finalize
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
	if state.Closing {
		return
	}
	if state.GoalAccountingActivators == nil {
		state.GoalAccountingActivators = make(map[string]GoalAccountingActivate)
	}
	if activate == nil {
		delete(state.GoalAccountingActivators, roundID)
		return
	}
	state.GoalAccountingActivators[roundID] = activate
}

// RegisterGoalAccountingCreateGuard 注册或移除 live runtime scope 的 Goal 创建保护。
// roundID 是 callback 生命周期键；scopeRoundID 是 DM round 或 Room root round 的计量边界。
func (m *Manager) RegisterGoalAccountingCreateGuard(
	sessionKey string,
	roundID string,
	scopeRoundID string,
	consumed GoalAccountingConsumed,
) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	scopeRoundID = strings.TrimSpace(scopeRoundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.GoalAccountingGuards == nil {
		state.GoalAccountingGuards = make(map[string]goalAccountingGuard)
	}
	if consumed == nil {
		delete(state.GoalAccountingGuards, roundID)
		return
	}
	state.GoalAccountingGuards[roundID] = goalAccountingGuard{
		scopeRoundID: scopeRoundID,
		consumed:     consumed,
	}
}

// GoalAccountingCreateConflicts 返回会与新 Goal 争用 live runtime scope 的 round。
// scopeRoundID 非空时只检查同一 scope；为空时检查整个 session。
func (m *Manager) GoalAccountingCreateConflicts(sessionKey string, scopeRoundID string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	scopeRoundID = strings.TrimSpace(scopeRoundID)
	if sessionKey == "" {
		return nil
	}

	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingGuards) == 0 {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := slices.Sorted(maps.Keys(state.GoalAccountingGuards))
	guards := make([]goalAccountingGuard, 0, len(roundIDs))
	candidateRoundIDs := make([]string, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		guard := state.GoalAccountingGuards[roundID]
		if scopeRoundID != "" && guard.scopeRoundID != scopeRoundID {
			continue
		}
		candidateRoundIDs = append(candidateRoundIDs, roundID)
		guards = append(guards, guard)
	}
	m.mu.RUnlock()

	conflicts := make([]string, 0, len(candidateRoundIDs))
	for index, guard := range guards {
		if guard.consumed != nil && guard.consumed() {
			conflicts = append(conflicts, candidateRoundIDs[index])
		}
	}
	return conflicts
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

// ClearGoalAccountingRounds 只清理指定 round，用于多 round activation 部分成功后的回滚。
func (m *Manager) ClearGoalAccountingRounds(sessionKey string, requestedRoundIDs []string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" || len(requestedRoundIDs) == 0 {
		return nil
	}
	requested := make(map[string]struct{}, len(requestedRoundIDs))
	for _, roundID := range requestedRoundIDs {
		if roundID = strings.TrimSpace(roundID); roundID != "" {
			requested[roundID] = struct{}{}
		}
	}
	if len(requested) == 0 {
		return nil
	}

	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingClearers) == 0 {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := slices.Sorted(maps.Keys(requested))
	clearers := make([]GoalAccountingClear, 0, len(roundIDs))
	matchedRoundIDs := make([]string, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		clear, exists := state.GoalAccountingClearers[roundID]
		if !exists {
			continue
		}
		matchedRoundIDs = append(matchedRoundIDs, roundID)
		clearers = append(clearers, clear)
	}
	m.mu.RUnlock()

	cleared := make([]string, 0, len(matchedRoundIDs))
	for index, clear := range clearers {
		if clear == nil {
			continue
		}
		clear()
		cleared = append(cleared, matchedRoundIDs[index])
	}
	return cleared
}

// BeginGoalAccountingFinalizing 要求 complete Goal 的运行中 round 保留绑定，
// 直到各自 provider terminal 对账完成。
func (m *Manager) BeginGoalAccountingFinalizing(sessionKey string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingFinalizers) == 0 {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := slices.Sorted(maps.Keys(state.GoalAccountingFinalizers))
	finalizers := make([]GoalAccountingFinalize, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		finalizers = append(finalizers, state.GoalAccountingFinalizers[roundID])
	}
	m.mu.RUnlock()

	finalizing := make([]string, 0, len(roundIDs))
	for index, finalize := range finalizers {
		if finalize == nil {
			continue
		}
		if !finalize() {
			continue
		}
		finalizing = append(finalizing, roundIDs[index])
	}
	return finalizing
}

// ActivateGoalAccounting 要求指定 session 的运行中 round 从当前快照开始归属明确 Goal。
func (m *Manager) ActivateGoalAccounting(ctx context.Context, sessionKey string, goalID string) ([]string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	goalID = strings.TrimSpace(goalID)
	if sessionKey == "" || goalID == "" {
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
		if err := activate(ctx, goalID); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		activated = append(activated, roundIDs[index])
	}
	return activated, firstErr
}
