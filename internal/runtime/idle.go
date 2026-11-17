// INPUT: session 最后使用时间、启动/round/后台/子任务状态与回收阈值。
// OUTPUT: TTL 或压力触发的空闲 session 关闭及满足条件时的 owner 进程回收。
// POS: Manager 的批量空闲生命周期入口。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// CloseIdleSessions 回收超过空闲阈值且所有任务已完成的 SDK client。
func (m *Manager) CloseIdleSessions(ctx context.Context, idleFor time.Duration) (int, error) {
	if idleFor <= 0 {
		return 0, nil
	}
	return m.closeIdleSessions(ctx, idleFor, 0)
}

// idleFor 为零只用于资源压力回收；maxTargets 为零表示不限数量。
func (m *Manager) closeIdleSessions(ctx context.Context, idleFor time.Duration, maxTargets int) (int, error) {
	now := m.nowTime().UTC()
	targets := make([]*sessionCloseTarget, 0)
	owners := make(map[string]struct{})

	m.mu.Lock()
	keys := m.idleSessionKeysLocked(now, idleFor)
	if maxTargets > 0 && len(keys) > maxTargets {
		keys = keys[:maxTargets]
	}
	for _, sessionKey := range keys {
		target, started, _ := m.beginSessionCloseLocked(sessionKey)
		if !started {
			continue
		}
		targets = append(targets, target)
		if target.ownerUserID != "" {
			owners[target.ownerUserID] = struct{}{}
		}
	}
	reapPlans := make([]*ownerReapPlan, 0, len(owners))
	for ownerUserID := range owners {
		if plan, _ := m.beginOwnerReapLocked(ownerUserID, nil, false); plan != nil {
			reapPlans = append(reapPlans, plan)
		}
	}
	m.mu.Unlock()

	for _, target := range targets {
		cancelSessionCloseTarget(target)
	}
	for _, plan := range reapPlans {
		m.startOwnerReap(plan)
	}

	errs := make([]error, 0, len(targets)+len(reapPlans))
	for _, target := range targets {
		var disconnectErr error
		if target.client != nil {
			disconnectCtx, cancel := context.WithTimeout(ctx, RoundIdleAbortTimeout)
			disconnectErr = target.client.Disconnect(disconnectCtx)
			cancel()
		}
		idleDrainErr := waitIdleMessageDrain(ctx, target.idleMessageDrain)
		backgroundErr := waitBackgroundTasks(ctx, target.backgroundDone)
		roundErr := waitRoundDoneForClose(ctx, target.roundDone)
		cleanupErr := errors.Join(disconnectErr, idleDrainErr, backgroundErr, roundErr)
		clientCleanupPending := errors.Is(disconnectErr, context.Canceled) ||
			errors.Is(disconnectErr, context.DeadlineExceeded)
		if clientCleanupPending || idleDrainErr != nil || backgroundErr != nil || roundErr != nil {
			m.finishSessionCloseWhenDone(target, clientCleanupPending, cleanupErr)
		} else {
			m.finishSessionClose(target, cleanupErr)
		}
		err := cleanupErr
		if err != nil && !IsRuntimeTransportClosedError(err) {
			errs = append(errs, fmt.Errorf("close idle runtime session %s: %w", target.sessionKey, err))
		}
	}
	for _, plan := range reapPlans {
		if err := waitOwnerReap(ctx, plan.flight); err != nil {
			errs = append(errs, fmt.Errorf("reap owner runtime processes: %w", err))
		}
	}
	return len(targets), errors.Join(errs...)
}

func (m *Manager) idleSessionKeysLocked(now time.Time, idleFor time.Duration) []string {
	var keys []string
	for key, state := range m.sessions {
		if state == nil || state.Closing || m.startupGates[key] != nil ||
			state.Rounds.active() || len(state.Rounds.doneSignals()) > 0 ||
			len(state.BackgroundTasks) > 0 || len(state.SubagentHooks) > 0 || activeSubagentBindings(state) > 0 {
			continue
		}
		if state.LastUsedAt.IsZero() {
			state.LastUsedAt = now
		}
		if now.Sub(state.LastUsedAt) < idleFor || (idleFor == 0 && state.Client == nil) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := m.sessions[keys[i]].LastUsedAt, m.sessions[keys[j]].LastUsedAt
		if a.Equal(b) {
			return keys[i] < keys[j]
		}
		return a.Before(b)
	})
	return keys
}
