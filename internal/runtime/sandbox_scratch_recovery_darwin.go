//go:build darwin

// INPUT: 持锁宿主、已收口进程 key、可信 scratch 证明与持久清理阶段。
// OUTPUT: 原目录先移入受保护回收区，再删除；重试只处理原记录。
// POS: 显式 macOS 资源恢复，不重放任务，不清除独立策略/业务 unknown。
package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sandboxScratchRecoveryStore interface {
	SandboxProcessStore
	PrepareScratchRecovery(context.Context, protocol.SandboxProcessKey) (protocol.SandboxScratchRecovery, error)
	AdvanceScratchRecovery(context.Context, protocol.SandboxScratchRecovery, protocol.SandboxScratchRecoveryPhase) error
}

// RecoverSandboxScratch requires native process recovery first. The configured
// supervisor root must be task-inaccessible, as required by its setup contract.
// A completed result concerns scratch only; policy and business facts are separate.
func (m *Manager) RecoverSandboxScratch(ctx context.Context, key protocol.SandboxProcessKey, ownership SandboxProcessRecoveryOwnership) (protocol.SandboxScratchRecovery, error) {
	result := protocol.SandboxScratchRecovery{}
	if ownership == nil {
		return result, errors.New("scratch recovery requires exclusive host ownership")
	}
	err := ownership.WithOwnership(func(appRoot string) error {
		startup, err := m.BeginClientStartup(ctx, key.SessionKey, key.OwnerUserID)
		if err != nil {
			return err
		}
		defer startup.Close()
		m.mu.RLock()
		config := m.sandboxSupervisor
		store, ok := m.sandboxReceiptStore.(sandboxScratchRecoveryStore)
		state := m.sessions[key.SessionKey]
		active := state != nil && (state.Client != nil || state.Closing)
		m.mu.RUnlock()
		if active || config == nil || !ok {
			return errors.New("scratch recovery requires inactive runtime and durable supervisor")
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
		process, found, err := store.Process(ctx, key)
		if err != nil {
			return err
		}
		if !found || process.Intent.Key != key {
			return errors.New("original scratch process missing")
		}
		if sandboxResourceIsActive(filepath.Join(process.Intent.Scratch.BasePath, process.Intent.Scratch.LeafName)) {
			return errors.New("scratch recovery cannot retire an in-process lease")
		}
		result, err = store.PrepareScratchRecovery(ctx, key)
		if err != nil {
			return err
		}
		if result.Phase == protocol.SandboxScratchRecoveryComplete {
			return nil
		}
		if sandboxResourceIsActive(filepath.Join(result.Scratch.BasePath, result.Scratch.LeafName)) {
			return errors.New("scratch recovery cannot retire an in-process lease")
		}
		return recoverScratchFiles(ctx, appRoot, config.Root, store, &result)
	})
	return result, err
}

func recoverScratchFiles(ctx context.Context, appRoot string, processRoot *confinedfs.Root, store sandboxScratchRecoveryStore, record *protocol.SandboxScratchRecovery) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	statePath, err := filepath.EvalSymlinks(filepath.Dir(appRoot))
	if err != nil {
		return err
	}
	expectedBase := filepath.Join(statePath, "users", safeOwnerPathSegment(record.ProcessKey.OwnerUserID), "runtime", scratchDirName)
	if record.Scratch.BasePath != expectedBase {
		return errors.New("scratch recovery path is outside owned canonical runtime")
	}
	quarantine, err := processRoot.OpenOrCreateRootNoSymlink("scratch-recovery", 0700)
	if err != nil {
		return err
	}
	defer quarantine.Close()
	target := "s-" + record.LeaseID
	advance := func(next protocol.SandboxScratchRecoveryPhase) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := store.AdvanceScratchRecovery(ctx, *record, next); err != nil {
			return err
		}
		record.Phase = next
		return nil
	}
	if record.Phase == protocol.SandboxScratchRecoveryPrepared {
		state, err := confinedfs.Open(statePath)
		if err != nil {
			return err
		}
		defer state.Close()
		base, err := state.OpenRootNoSymlink(filepath.Join("users", safeOwnerPathSegment(record.ProcessKey.OwnerUserID), "runtime", scratchDirName))
		if err != nil {
			return err
		}
		defer base.Close()
		info, err := base.Stat(".")
		if err != nil {
			return err
		}
		identity, err := sandboxDirectoryIdentity(info)
		if err != nil || identity != record.Scratch.BaseIdentity {
			return errors.New("scratch recovery parent identity mismatch")
		}
		_, err = verifiedRecoveryDirectory(quarantine, target, record.Scratch.LeafIdentity)
		if errors.Is(err, os.ErrNotExist) {
			original, err := verifiedRecoveryDirectory(base, record.Scratch.LeafName, record.Scratch.LeafIdentity)
			if err != nil {
				return err
			}
			if err := base.QuarantineDirectory(record.Scratch.LeafName, quarantine, target, original); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		// Also runs after a host crash between rename and its durable phase update.
		if _, err := verifiedRecoveryDirectory(quarantine, target, record.Scratch.LeafIdentity); err != nil {
			return err
		}
		if err := errors.Join(base.SyncDirectory(), quarantine.SyncDirectory()); err != nil {
			return err
		}
		if err := advance(protocol.SandboxScratchRecoveryQuarantined); err != nil {
			return err
		}
	}
	if record.Phase == protocol.SandboxScratchRecoveryQuarantined {
		if _, err := verifiedRecoveryDirectory(quarantine, target, record.Scratch.LeafIdentity); err != nil {
			return err
		}
		if err := advance(protocol.SandboxScratchRecoveryDeleting); err != nil {
			return err
		}
	}
	if record.Phase == protocol.SandboxScratchRecoveryDeleting {
		_, err := verifiedRecoveryDirectory(quarantine, target, record.Scratch.LeafIdentity)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil {
			if err := quarantine.RemoveAll(target); err != nil {
				return err
			}
		}
		// Absence is success only after the durable deleting phase in this protected
		// root. A missing source in prepared state is never interpreted as deletion.
		if err := quarantine.SyncDirectory(); err != nil {
			return err
		}
		if err := advance(protocol.SandboxScratchRecoveryComplete); err != nil {
			return err
		}
	}
	if record.Phase != protocol.SandboxScratchRecoveryComplete {
		return errors.New("unknown scratch recovery phase")
	}
	return nil
}

func verifiedRecoveryDirectory(root *confinedfs.Root, name, expected string) (os.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	identity, err := sandboxDirectoryIdentity(info)
	if err != nil || identity != expected {
		return nil, errors.New("scratch recovery directory identity mismatch")
	}
	return info, nil
}
