// INPUT: 原Windows live scratch lease、固定scope/代次与独立Windows持久事实。
// OUTPUT: 只有完整cleaned后才删除原scratch；unknown保留原owner和marker。
// POS: 正常Release的资源栅栏，不伪造Windows崩溃恢复或Darwin隔离回收区。
package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// configureWindowsSandboxLeaseCleanup 将同一资源的最后Release绑定到原Windows监督来源，不允许更换所有者。
func configureWindowsSandboxLeaseCleanup(lease *SandboxResourceLease, config *WindowsSandboxSupervisor, store WindowsSandboxStore, binding windowsSandboxBinding) error {
	if lease == nil {
		return errors.New("Windows supervised execution requires its exact resource lease")
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released || lease.resource == nil {
		return errors.New("Windows scratch lease is no longer active")
	}
	resource := lease.resource
	resource.mu.Lock()
	defer resource.mu.Unlock()
	if resource.windowsCleanupSupervisor != nil {
		if resource.windowsCleanupSupervisor != config {
			return errors.New("Windows scratch cannot replace its original supervisor")
		}
		return nil
	}
	if resource.supervisedCleanup != nil || resource.cleanupSupervisor != nil {
		return errors.New("Windows scratch cannot borrow another platform cleanup")
	}
	resource.windowsCleanupSupervisor = config
	var removed bool
	var removedKey protocol.WindowsSandboxKey
	resource.supervisedCleanup = func(resource *sandboxResource) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Release holds resource.mu. After a successful native removal, retry
		// only the exact durable completion; absence of a path is never proof.
		if removed {
			return store.CompleteWindowsSandboxResources(ctx, removedKey)
		}
		latest, found, err := store.LatestWindowsSandbox(ctx, binding.Owner, binding.Session)
		if err != nil {
			return err
		}
		if !found || latest.Intent.Key.Generation < binding.Generation {
			return removeSandboxResource(resource)
		}
		if latest.Intent.Key.OwnerUserID != binding.Owner || latest.Intent.Key.SessionKey != binding.Session || latest.Intent.LeaseID != binding.LeaseID {
			return errors.New("Windows scratch no longer matches original persistent launch scope")
		}
		if latest.Phase != "cleaned" || latest.Outcome == nil || !latest.Outcome.Cleaned || latest.Prepared == nil || latest.Outcome.Prepared != *latest.Prepared {
			return ErrSandboxCleanupPending
		}
		if latest.Intent.ScratchRoot != binding.ScratchRoot {
			return ErrSandboxCleanupPending
		}
		if err := removeSandboxResource(resource); err != nil {
			return err
		}
		removed, removedKey = true, latest.Intent.Key
		return store.CompleteWindowsSandboxResources(ctx, removedKey)
	}
	return nil
}
