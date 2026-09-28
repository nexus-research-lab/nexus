// INPUT: 持锁宿主与原进程 exact key。
// OUTPUT: 进程和资源证据均完整后，只对关联策略记录提交 reconciled。
// POS: 策略恢复入口，不以最新代次猜测，不更新或重放业务操作。
package runtime

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sandboxPolicyRecoveryStore interface {
	ReconcileProcessPolicies(context.Context, protocol.SandboxProcessKey) (int64, error)
}

// ReconcileSandboxPolicy runs after native and (when present) scratch recovery.
// A zero count leaves historical unbound receipts untouched; it is not proof
// that every receipt in the session has been recovered.
func (m *Manager) ReconcileSandboxPolicy(ctx context.Context, key protocol.SandboxProcessKey, ownership SandboxProcessRecoveryOwnership) (int64, error) {
	var count int64
	if ownership == nil {
		return 0, errors.New("policy recovery requires exclusive host ownership")
	}
	err := ownership.WithOwnership(func(appRoot string) error {
		var err error
		count, err = m.reconcileSandboxPolicyOwned(ctx, key, appRoot)
		return err
	})
	return count, err
}

func (m *Manager) reconcileSandboxPolicyOwned(ctx context.Context, key protocol.SandboxProcessKey, appRoot string) (int64, error) {
	var count int64
	err := func() error {
		startup, err := m.BeginClientStartup(ctx, key.SessionKey, key.OwnerUserID)
		if err != nil {
			return err
		}
		defer startup.Close()
		m.mu.RLock()
		config := m.sandboxSupervisor
		store, ok := m.sandboxReceiptStore.(sandboxPolicyRecoveryStore)
		state := m.sessions[key.SessionKey]
		active := state != nil && (state.Client != nil || state.Closing)
		m.mu.RUnlock()
		if active || config == nil || !ok {
			return errors.New("policy recovery requires inactive runtime and durable supervisor")
		}
		if err := verifyRecoveryRoot(appRoot, config.Root); err != nil {
			return err
		}
		count, err = store.ReconcileProcessPolicies(ctx, key)
		return err
	}()
	return count, err
}
