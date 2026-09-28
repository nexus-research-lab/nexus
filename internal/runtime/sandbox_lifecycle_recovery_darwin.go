//go:build darwin

// INPUT: 已完成原生进程扫描的持锁宿主与稳定 launch ID 游标。
// OUTPUT: 有界恢复已终止进程的资源和策略步骤；失败仍可重新扫描。
// POS: App 开放任务准入前的第二阶段；不重放任何任务。
package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sandboxLifecycleRecoveryStore interface {
	SandboxProcessStore
	PendingLifecycleKeys(context.Context, string, int) ([]protocol.SandboxProcessKey, bool, error)
	CompleteProcessResources(context.Context, protocol.SandboxProcessKey) error
}

type SandboxLifecycleRecoveryItem struct {
	Key                protocol.SandboxProcessKey
	ResourcesComplete  bool
	PoliciesReconciled int64
	Err                error
}
type SandboxLifecycleRecoveryBatch struct {
	Items      []SandboxLifecycleRecoveryItem
	NextCursor string
	HasMore    bool
}

// RecoverPendingSandboxLifecycles runs after all native recovery pages, before
// admitting tasks. HasMore=false never cancels an item error; retry failed
// records from the beginning on a later pass, retaining their original keys.
func (m *Manager) RecoverPendingSandboxLifecycles(ctx context.Context, ownership SandboxProcessRecoveryOwnership, after string, limit int) (SandboxLifecycleRecoveryBatch, error) {
	result := SandboxLifecycleRecoveryBatch{NextCursor: after}
	if ownership == nil || limit < 1 || limit > 256 || (after != "" && !validSandboxLaunchID(after)) {
		return result, errors.New("invalid lifecycle recovery admission")
	}
	err := ownership.WithOwnership(func(appRoot string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		m.mu.RLock()
		config := m.sandboxSupervisor
		store, ok := m.sandboxReceiptStore.(sandboxLifecycleRecoveryStore)
		m.mu.RUnlock()
		if config == nil || !ok {
			return errors.New("lifecycle recovery requires durable supervisor")
		}
		if err := verifyRecoveryRoot(appRoot, config.Root); err != nil {
			return err
		}
		keys, more, err := store.PendingLifecycleKeys(ctx, after, limit)
		if err != nil {
			return err
		}
		result.HasMore = more
		var failures []error
		for _, key := range keys {
			if err := ctx.Err(); err != nil {
				result.HasMore = true
				return errors.Join(append(failures, err)...)
			}
			if !validSandboxLaunchID(key.LaunchID) || key.LaunchID <= result.NextCursor {
				result.HasMore = true
				return errors.Join(append(failures, errors.New("invalid lifecycle recovery cursor order"))...)
			}
			item := SandboxLifecycleRecoveryItem{Key: key}
			m.mu.RLock()
			state := m.sessions[key.SessionKey]
			active := state != nil && (state.Client != nil || state.Closing)
			m.mu.RUnlock()
			process, found, err := store.Process(ctx, key)
			if err == nil && active {
				err = errors.New("cannot recover lifecycle owned by active runtime")
			}
			if err == nil && (!found || (process.Phase != protocol.SandboxProcessReaped && process.Phase != protocol.SandboxProcessAborted)) {
				err = errors.New("lifecycle recovery requires original terminal process")
			}
			if err == nil && process.Intent.LeaseID != "" {
				_, err = m.recoverSandboxScratchOwned(ctx, key, appRoot)
			}
			if err == nil {
				err = store.CompleteProcessResources(ctx, key)
				item.ResourcesComplete = err == nil
			}
			if err == nil && process.Phase == protocol.SandboxProcessReaped {
				item.PoliciesReconciled, err = m.reconcileSandboxPolicyOwned(ctx, key, appRoot)
			}
			item.Err = err
			result.Items = append(result.Items, item)
			result.NextCursor = key.LaunchID
			if err != nil {
				failures = append(failures, fmt.Errorf("recover lifecycle %s: %w", key.LaunchID, err))
			}
		}
		return errors.Join(failures...)
	})
	return result, err
}
