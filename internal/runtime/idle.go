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
		sandboxPhaseErr := m.markSandboxReceiptRetiring(target)
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
		sandboxTerminalPhaseErr := m.finalizeSandboxReceipt(target, cleanupErr)
		cleanupErr = errors.Join(cleanupErr, sandboxPhaseErr, sandboxTerminalPhaseErr)
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
	reaperErrors := make(map[string]error, len(reapPlans))
	for _, plan := range reapPlans {
		if err := waitOwnerReap(ctx, plan.flight); err != nil {
			reaperErrors[plan.ownerUserID] = err
			errs = append(errs, fmt.Errorf("reap owner runtime processes: %w", err))
		}
	}
	for _, target := range targets {
		if err := reaperErrors[target.ownerUserID]; err != nil {
			if receiptErr := m.markSandboxReceiptUnknownAfterReaper(target, err); receiptErr != nil {
				errs = append(errs, fmt.Errorf("record sandbox reaper uncertainty: %w", receiptErr))
			}
		}
	}
	return len(targets), errors.Join(errs...)
}
