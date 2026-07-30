// INPUT: runtime session 中持久的 owner_user_id。
// OUTPUT: owner 级热会话回收，用于权限降级后撤销现存进程凭据。
// POS: Manager 的跨会话安全生命周期入口。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// CloseOwnerSessions 关闭指定 owner 的全部 runtime，并取消仍在执行的 round。
func (m *Manager) CloseOwnerSessions(ctx context.Context, ownerUserID string) (int, error) {
	if m == nil || strings.TrimSpace(ownerUserID) == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	targets := make([]*sessionCloseTarget, 0)
	var reaperErr error

	m.mu.Lock()
	for sessionKey, state := range m.sessions {
		if state == nil || state.OwnerUserID != ownerUserID {
			continue
		}
		target, started, _ := m.beginSessionCloseLocked(sessionKey)
		if started {
			targets = append(targets, target)
		}
	}
	// 回收动作在释放 Manager 锁前执行，阻止新的同 owner session 在
	// cgroup.kill 与旧 session 清理之间插入。
	reaperErr = m.reapOwnerIfLastLocked(ctx, ownerUserID)
	m.mu.Unlock()

	errs := make([]error, 0, len(targets)+1)
	if reaperErr != nil {
		errs = append(errs, fmt.Errorf("reap owner runtime processes: %w", reaperErr))
	}
	for _, target := range targets {
		if target.idleMessageCancel != nil {
			target.idleMessageCancel()
		}
		for _, cancel := range target.roundCancels {
			if cancel != nil {
				cancel()
			}
		}
		for _, cancel := range target.backgroundCancels {
			if cancel != nil {
				cancel()
			}
		}
		var err error
		if target.client != nil {
			disconnectCtx, cancel := context.WithTimeout(ctx, RoundIdleAbortTimeout)
			err = target.client.Disconnect(disconnectCtx)
			cancel()
		}
		backgroundErr := waitBackgroundTasks(ctx, target.backgroundDone)
		if backgroundErr != nil {
			err = errors.Join(err, backgroundErr)
		}
		roundErr := waitRoundDoneForClose(ctx, target.roundDone)
		if roundErr != nil {
			err = errors.Join(err, roundErr)
		}
		if backgroundErr != nil || roundErr != nil {
			m.finishSessionCloseWhenDone(target)
		} else {
			m.finishSessionClose(target)
		}
		if err != nil && !IsRuntimeTransportClosedError(err) {
			errs = append(errs, fmt.Errorf(
				"close owner runtime session %s: %w",
				target.sessionKey,
				err,
			))
		}
	}
	return len(targets), errors.Join(errs...)
}

func (m *Manager) reapOwnerIfLastLocked(ctx context.Context, ownerUserID string) error {
	if m == nil || m.ownerProcessReaper == nil || strings.TrimSpace(ownerUserID) == "" {
		return nil
	}
	for _, state := range m.sessions {
		if state != nil && !state.Closing && state.Client != nil && state.OwnerUserID == ownerUserID {
			return nil
		}
	}
	return m.ownerProcessReaper.ReapOwnerProcesses(ctx, ownerUserID)
}
