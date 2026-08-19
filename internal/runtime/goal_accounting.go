// INPUT: 运行中 round 的 Goal accounting 回调与 objective revision 指针。
// OUTPUT: session/round 级结算、清理、激活和 revision adoption。
// POS: runtime Manager 中 Goal 执行态的注册与并发协调入口。
package runtime

import (
	"context"
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

// GoalAccountingIdentity reports the exact Goal currently owned by one live
// round. It is read under no Manager lock and must be concurrency-safe.
type GoalAccountingIdentity func() string

type goalAccountingGuard struct {
	scopeRoundID string
	consumed     GoalAccountingConsumed
}

// updateGoalAccountingHooks 统一维护 round 的可选 Goal accounting 能力。
func (m *Manager) updateGoalAccountingHooks(
	sessionKey string,
	roundID string,
	present bool,
	update func(*goalAccountingHooks),
) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" || update == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil && present {
		state = m.ensureStateLocked(sessionKey)
	}
	if state == nil || state.Closing {
		return
	}
	round := state.Rounds.get(roundID)
	if present {
		round = state.Rounds.ensure(roundID)
	}
	if round == nil {
		return
	}
	update(&round.goal)
	state.Rounds.prune(roundID)
	m.removeClientlessSessionIfIdleLocked(sessionKey, state, nil)
}

// RegisterGoalObjectiveRevision 让运行中 round 的 command 与终态回调共享同一 objective revision。
func (m *Manager) RegisterGoalObjectiveRevision(sessionKey string, roundID string, revision *atomic.Int64) {
	m.updateGoalAccountingHooks(sessionKey, roundID, revision != nil, func(hooks *goalAccountingHooks) {
		hooks.objectiveRevision = revision
	})
}

// RegisterGoalAccountingIdentity registers the exact Goal identity currently
// accounted by one live round. Empty means the round has no Goal authority.
func (m *Manager) RegisterGoalAccountingIdentity(
	sessionKey string,
	roundID string,
	identity GoalAccountingIdentity,
) {
	m.updateGoalAccountingHooks(sessionKey, roundID, identity != nil, func(hooks *goalAccountingHooks) {
		hooks.goalID = identity
	})
}

// GoalAccountingRoundIDs returns only live rounds that currently account the
// requested Goal. It never broadens to every round in the session.
func (m *Manager) GoalAccountingRoundIDs(sessionKey string, goalID string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	goalID = strings.TrimSpace(goalID)
	if sessionKey == "" || goalID == "" {
		return nil
	}
	m.mu.RLock()
	state := m.sessions[sessionKey]
	if state == nil {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := state.Rounds.matchingIDs(func(round *roundState) bool {
		return round.goal.goalID != nil
	})
	identities := make([]GoalAccountingIdentity, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		identities = append(identities, state.Rounds.get(roundID).goal.goalID)
	}
	m.mu.RUnlock()

	matched := make([]string, 0, len(roundIDs))
	for index, identity := range identities {
		if identity != nil && strings.TrimSpace(identity()) == goalID {
			matched = append(matched, roundIDs[index])
		}
	}
	return matched
}

// AdoptGoalObjectiveRevision 在 steering 真正被 runtime 消费后推进运行中 round 的 revision fence。
func (m *Manager) AdoptGoalObjectiveRevision(sessionKey string, revision int64) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" || revision <= 0 {
		return nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := state.Rounds.matchingIDs(func(round *roundState) bool {
		return round.goal.objectiveRevision != nil
	})
	revisions := make([]*atomic.Int64, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		revisions = append(revisions, state.Rounds.get(roundID).goal.objectiveRevision)
	}
	m.mu.RUnlock()

	adopted := make([]string, 0, len(roundIDs))
	for index, objectiveRevision := range revisions {
		// revision=0 表示该 round 尚未获得 exact Goal authority，不能被
		// session 级 steering 顺带提升为已授权状态。
		if objectiveRevision == nil || objectiveRevision.Load() <= 0 {
			continue
		}
		for {
			current := objectiveRevision.Load()
			if revision <= current || objectiveRevision.CompareAndSwap(current, revision) {
				break
			}
		}
		adopted = append(adopted, roundIDs[index])
	}
	return adopted
}

// RegisterGoalAccountingFlush 注册或移除运行中 round 的 Goal accounting flush 回调。
func (m *Manager) RegisterGoalAccountingFlush(sessionKey string, roundID string, flush GoalAccountingFlush) {
	m.updateGoalAccountingHooks(sessionKey, roundID, flush != nil, func(hooks *goalAccountingHooks) {
		hooks.flush = flush
	})
}

// RegisterGoalAccountingClear 注册或移除运行中 round 的 Goal accounting clear 回调。
func (m *Manager) RegisterGoalAccountingClear(sessionKey string, roundID string, clear GoalAccountingClear) {
	m.updateGoalAccountingHooks(sessionKey, roundID, clear != nil, func(hooks *goalAccountingHooks) {
		hooks.clear = clear
	})
}

// RegisterGoalAccountingFinalize 注册或移除运行中 round 的 Goal terminal 对账回调。
func (m *Manager) RegisterGoalAccountingFinalize(
	sessionKey string,
	roundID string,
	finalize GoalAccountingFinalize,
) {
	m.updateGoalAccountingHooks(sessionKey, roundID, finalize != nil, func(hooks *goalAccountingHooks) {
		hooks.finalize = finalize
	})
}

// RegisterGoalAccountingActivate 注册或移除运行中 round 的 Goal accounting active 回调。
func (m *Manager) RegisterGoalAccountingActivate(sessionKey string, roundID string, activate GoalAccountingActivate) {
	m.updateGoalAccountingHooks(sessionKey, roundID, activate != nil, func(hooks *goalAccountingHooks) {
		hooks.activate = activate
	})
}

// RegisterGoalAccountingCreateGuard 注册或移除 live runtime scope 的 Goal 创建保护。
// roundID 是 callback 生命周期键；scopeRoundID 是 DM round 或 Room root round 的计量边界。
func (m *Manager) RegisterGoalAccountingCreateGuard(
	sessionKey string,
	roundID string,
	scopeRoundID string,
	consumed GoalAccountingConsumed,
) {
	scopeRoundID = strings.TrimSpace(scopeRoundID)
	m.updateGoalAccountingHooks(sessionKey, roundID, consumed != nil, func(hooks *goalAccountingHooks) {
		hooks.guard = goalAccountingGuard{
			scopeRoundID: scopeRoundID,
			consumed:     consumed,
		}
	})
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
	if !ok || state == nil {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := state.Rounds.matchingIDs(func(round *roundState) bool {
		guard := round.goal.guard
		return guard.consumed != nil && (scopeRoundID == "" || guard.scopeRoundID == scopeRoundID)
	})
	guards := make([]goalAccountingGuard, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		guards = append(guards, state.Rounds.get(roundID).goal.guard)
	}
	m.mu.RUnlock()

	conflicts := make([]string, 0, len(roundIDs))
	for index, guard := range guards {
		if guard.consumed() {
			conflicts = append(conflicts, roundIDs[index])
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
	if !ok || state == nil {
		m.mu.RUnlock()
		return nil, nil
	}
	roundIDs := state.Rounds.matchingIDs(func(round *roundState) bool {
		return round.goal.flush != nil
	})
	flushers := make([]GoalAccountingFlush, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		flushers = append(flushers, state.Rounds.get(roundID).goal.flush)
	}
	m.mu.RUnlock()

	var firstErr error
	flushed := make([]string, 0, len(roundIDs))
	for index, flush := range flushers {
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
	if !ok || state == nil {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := state.Rounds.matchingIDs(func(round *roundState) bool {
		return round.goal.clear != nil
	})
	clearers := make([]GoalAccountingClear, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		clearers = append(clearers, state.Rounds.get(roundID).goal.clear)
	}
	m.mu.RUnlock()

	cleared := make([]string, 0, len(roundIDs))
	for index, clear := range clearers {
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
	roundIDs := make([]string, 0, len(requested))
	for roundID := range requested {
		roundIDs = append(roundIDs, roundID)
	}
	slices.Sort(roundIDs)

	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil {
		m.mu.RUnlock()
		return nil
	}
	clearers := make([]GoalAccountingClear, 0, len(roundIDs))
	matchedRoundIDs := make([]string, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		round := state.Rounds.get(roundID)
		if round == nil || round.goal.clear == nil {
			continue
		}
		matchedRoundIDs = append(matchedRoundIDs, roundID)
		clearers = append(clearers, round.goal.clear)
	}
	m.mu.RUnlock()

	for _, clear := range clearers {
		clear()
	}
	return matchedRoundIDs
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
	if !ok || state == nil {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := state.Rounds.matchingIDs(func(round *roundState) bool {
		return round.goal.finalize != nil
	})
	finalizers := make([]GoalAccountingFinalize, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		finalizers = append(finalizers, state.Rounds.get(roundID).goal.finalize)
	}
	m.mu.RUnlock()

	finalizing := make([]string, 0, len(roundIDs))
	for index, finalize := range finalizers {
		if finalize() {
			finalizing = append(finalizing, roundIDs[index])
		}
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
	if !ok || state == nil {
		m.mu.RUnlock()
		return nil, nil
	}
	roundIDs := state.Rounds.matchingIDs(func(round *roundState) bool {
		return round.goal.activate != nil
	})
	activators := make([]GoalAccountingActivate, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		activators = append(activators, state.Rounds.get(roundID).goal.activate)
	}
	m.mu.RUnlock()

	var firstErr error
	activated := make([]string, 0, len(roundIDs))
	for index, activate := range activators {
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
