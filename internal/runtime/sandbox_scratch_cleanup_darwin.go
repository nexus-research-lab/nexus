//go:build darwin

// INPUT: 原 live lease、冻结的 supervisor/数据库和原进程代次下界。
// OUTPUT: 正常最后一次 Release 也使用持久回收区清理，响应丢失可续。
// POS: 热退出与冷恢复共用清理事实；不把路径缺失当成未登记进程的证明。
package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

type sandboxManagedScratchStore interface {
	sandboxScratchRecoveryStore
	SandboxProcessReceiptReader
}

func configureSupervisedLeaseCleanup(lease *SandboxResourceLease, config *SandboxProcessSupervisor, store SandboxProcessStore, binding sandboxProcessBinding) error {
	if lease == nil {
		return nil
	}
	durable, ok := store.(sandboxManagedScratchStore)
	if !ok {
		return errors.New("supervised scratch requires durable cleanup store")
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released || lease.resource == nil {
		return errors.New("supervised cleanup requires live lease")
	}
	resource := lease.resource
	resource.mu.Lock()
	defer resource.mu.Unlock()
	if resource.cleanupSupervisor != nil {
		if resource.cleanupSupervisor != config {
			return errors.New("scratch cannot change original supervisor")
		}
		return nil
	}
	resource.cleanupSupervisor = config
	// The original scope and generation floor are immutable even when clients
	// are replaced while other handles still keep this same resource alive.
	resource.supervisedCleanup = func(resource *sandboxResource) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		process, found, err := durable.LatestProcess(ctx, binding.Owner, binding.Session)
		if err != nil {
			return err
		}
		if !found || process.Intent.Key.Generation < binding.Generation {
			// Reserve precedes every supervised launch. No registration at or above
			// this client floor proves this resource was never exposed to a process.
			return removeSandboxResource(resource)
		}
		if process.Intent.Key.OwnerUserID != binding.Owner || process.Intent.Key.SessionKey != binding.Session || process.Intent.LeaseID != binding.LeaseID || process.Intent.Scratch != binding.Scratch {
			return errors.New("original supervised scratch launch does not match cleanup")
		}
		record, err := durable.PrepareScratchRecovery(ctx, process.Intent.Key)
		if err != nil {
			return err
		}
		return resumeSandboxScratchDeletion(ctx, config.Root, durable, &record, func() (*confinedfs.Root, error) { return confinedfs.Open(resource.root) })
	}
	return nil
}
