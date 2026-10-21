//go:build darwin

// INPUT: owner、启动恢复记下的 launch ID 与显式理由；宿主持锁所有权。
// OUTPUT: 已终止进程的资源/策略栅栏收口，会话恢复可用。
// POS: 自动恢复无法完成时的 owner 出口；只接受 lifecycle 阶段，不处理可能存活的进程。
package runtime

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrSandboxRecoveryNotFound means the launch is not a recorded startup
// recovery failure of this owner.
var ErrSandboxRecoveryNotFound = errors.New("sandbox startup recovery record not found")

// ErrSandboxRecoveryNotDiscardable means the record's process may still be
// alive; it keeps fencing only its own session.
var ErrSandboxRecoveryNotDiscardable = errors.New("sandbox startup recovery record cannot be discarded")

type sandboxLifecycleDiscardStore interface {
	DiscardProcessLifecycle(context.Context, protocol.SandboxProcessKey, string) (int64, error)
}

// DiscardSandboxStartupRecovery closes the fences of one lifecycle record that
// startup recovery could not finish. It is an explicit owner decision and never
// claims the scratch was deleted; leftover files stay for the marker sweep.
func (m *Manager) DiscardSandboxStartupRecovery(ctx context.Context, owner, launchID, reason string) error {
	owner = strings.TrimSpace(owner)
	failure, ownership, found := m.sandboxStartupRecoveryFailure(owner, strings.TrimSpace(launchID))
	if !found {
		return ErrSandboxRecoveryNotFound
	}
	if !failure.Discardable() {
		return ErrSandboxRecoveryNotDiscardable
	}
	if ownership == nil {
		return errors.New("sandbox recovery discard requires exclusive host ownership")
	}
	key := failure.Key
	var receipts int64
	err := ownership.WithOwnership(func(appRoot string) error {
		startup, err := m.BeginClientStartup(ctx, key.SessionKey, key.OwnerUserID)
		if err != nil {
			return err
		}
		defer startup.Close()
		m.mu.RLock()
		config := m.sandboxSupervisor
		store, ok := m.sandboxReceiptStore.(sandboxLifecycleDiscardStore)
		state := m.sessions[key.SessionKey]
		active := state != nil && (state.Client != nil || state.Closing)
		m.mu.RUnlock()
		if active || config == nil || !ok {
			return errors.New("sandbox recovery discard requires inactive runtime and durable supervisor")
		}
		if err := verifyRecoveryRoot(appRoot, config.Root); err != nil {
			return err
		}
		select {
		case acquisitionGate <- struct{}{}:
			defer func() { <-acquisitionGate }()
		case <-ctx.Done():
			return ctx.Err()
		}
		receipts, err = store.DiscardProcessLifecycle(ctx, key, reason)
		return err
	})
	if err != nil {
		return err
	}
	m.forgetSandboxStartupRecoveryFailure(key)
	slog.Warn("sandbox startup recovery record discarded by owner",
		"owner_user_id", key.OwnerUserID,
		"session_key", key.SessionKey,
		"launch_id", key.LaunchID,
		"previous_error", failure.Reason,
		"receipts_reconciled", receipts,
	)
	return nil
}
