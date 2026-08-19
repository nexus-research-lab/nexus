// INPUT: session 最后使用时间、活动 round 与空闲阈值。
// OUTPUT: 空闲 session 关闭及满足条件时的 owner 进程回收。
// POS: Manager 的批量空闲生命周期入口。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CloseIdleSessions 回收超过空闲阈值且没有运行中 round 的 SDK client。
func (m *Manager) CloseIdleSessions(ctx context.Context, idleFor time.Duration) (int, error) {
	if idleFor <= 0 {
		return 0, nil
	}

	now := m.nowTime().UTC()
	targets := make([]*sessionCloseTarget, 0)
	owners := make(map[string]struct{})

	m.mu.Lock()
	for sessionKey, state := range m.sessions {
		if state == nil || state.Closing || state.Rounds.active() {
			continue
		}
		lastUsedAt := state.LastUsedAt
		if lastUsedAt.IsZero() {
			state.LastUsedAt = now
			continue
		}
		if now.Sub(lastUsedAt) < idleFor {
			continue
		}
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
		clientCleanupPending := errors.Is(disconnectErr, context.Canceled) ||
			errors.Is(disconnectErr, context.DeadlineExceeded)
		if clientCleanupPending || idleDrainErr != nil || backgroundErr != nil || roundErr != nil {
			m.finishSessionCloseWhenDone(target, clientCleanupPending)
		} else {
			m.finishSessionClose(target)
		}
		err := errors.Join(disconnectErr, idleDrainErr, backgroundErr, roundErr)
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
