// INPUT: 原Windows launch key、宿主独占所有权与不可变持久终态。
// OUTPUT: 可查询诊断及精确策略对账；缺原生证明继续保留unknown。
// POS: 冷恢复不得用PID消失或路径不存在代替真实清理回执。
package runtime

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type windowsSandboxPolicyReconciler interface {
	ReconcileWindowsSandboxPolicies(context.Context, protocol.WindowsSandboxKey) (int64, error)
}

func (m *Manager) InspectWindowsSandboxRecovery(ctx context.Context, key protocol.WindowsSandboxKey) (protocol.WindowsSandboxRecoveryDiagnostic, error) {
	var d protocol.WindowsSandboxRecoveryDiagnostic
	m.mu.RLock()
	store, ok := m.sandboxReceiptStore.(WindowsSandboxStore)
	m.mu.RUnlock()
	if !ok {
		return d, errors.New("Windows recovery repository unavailable")
	}
	p, found, err := store.WindowsSandbox(ctx, key)
	if err != nil {
		return d, err
	}
	if !found || p.Intent.Key != key {
		return d, errors.New("exact Windows launch missing")
	}
	d.Process = p
	r, found, err := store.WindowsSandboxResources(ctx, key.OwnerUserID, key.SessionKey, p.Intent.LeaseID)
	if err != nil {
		return d, err
	}
	if found {
		d.Resources = &r
	}
	if p.Phase != "cleaned" || p.Prepared == nil || p.Outcome == nil || !p.Outcome.Cleaned || p.Outcome.Prepared != *p.Prepared {
		d.Reason = "native_execution_cleanup_proof_required"
		return d, nil
	}
	if !found || r.Phase != "complete" || r.ScratchRoot != p.Intent.ScratchRoot {
		d.Reason = "original_scratch_cleanup_proof_required"
		return d, nil
	}
	return d, nil
}

// RecoverWindowsSandbox only reconciles already proven terminal facts. It
// cannot manufacture the missing SDK native crash-recovery acknowledgement.
func (m *Manager) RecoverWindowsSandbox(ctx context.Context, key protocol.WindowsSandboxKey, ownership SandboxProcessRecoveryOwnership) (protocol.WindowsSandboxRecoveryDiagnostic, error) {
	var d protocol.WindowsSandboxRecoveryDiagnostic
	if ownership == nil {
		return d, errors.New("Windows recovery requires exclusive host ownership")
	}
	err := ownership.WithOwnership(func(appRoot string) error {
		gate, err := m.BeginClientStartup(ctx, key.SessionKey, key.OwnerUserID)
		if err != nil {
			return err
		}
		defer gate.Close()
		m.mu.RLock()
		config := m.windowsSandboxSupervisor
		store, ok := m.sandboxReceiptStore.(windowsSandboxPolicyReconciler)
		state := m.sessions[key.SessionKey]
		active := state != nil && (state.Client != nil || state.Closing)
		m.mu.RUnlock()
		if active || config == nil || !ok {
			return errors.New("Windows recovery requires configured inactive runtime")
		}
		if err := verifyRecoveryRoot(appRoot, config.Root); err != nil {
			return err
		}
		d, err = m.InspectWindowsSandboxRecovery(ctx, key)
		if err != nil {
			return err
		}
		if d.Reason != "" {
			return ErrSandboxCleanupPending
		}
		d.ReconciledPolicies, err = store.ReconcileWindowsSandboxPolicies(ctx, key)
		return err
	})
	return d, err
}
