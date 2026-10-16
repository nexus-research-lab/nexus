//go:build windows

// INPUT: SDK精确cleaned后的原scratch、固定回收区和写前持久阶段。
// OUTPUT: 原句柄移入回收区、删除证明及资源complete；未知缺失继续保留。
// POS: 冷恢复和正常Release共用原生路径，不使用RemoveAll路径重解析。
package runtime

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"path/filepath"
)

func cleanupWindowsScratch(ctx context.Context, root *confinedfs.Root, store WindowsSandboxStore, key protocol.WindowsSandboxKey, resource *sandboxResource, proof *windowsScratchLiveProof) error {
	durable, ok := store.(windowsScratchStore)
	if !ok {
		return errors.New("Windows scratch requires durable recovery repository")
	}
	_, err := resumeWindowsScratchCleanup(ctx, root, durable, key, proof, func(path string) (*confinedfs.Root, error) {
		if filepath.Dir(path) != resource.root || path != resource.path {
			return nil, errors.New("Windows scratch differs from live owner")
		}
		return confinedfs.Open(resource.root)
	})
	return err
}

func resumeWindowsScratchCleanup(ctx context.Context, root *confinedfs.Root, store windowsScratchStore, key protocol.WindowsSandboxKey, proof *windowsScratchLiveProof, openParent func(string) (*confinedfs.Root, error)) (protocol.WindowsScratchRecovery, error) {
	var record protocol.WindowsScratchRecovery
	if proof != nil && proof.Removed {
		record = proof.Record
		if record.Key != key {
			return record, errors.New("live Windows deletion proof differs from launch")
		}
		if record.Phase == "deleting" {
			if err := flushWindowsScratchRoots(root, record, openParent); err != nil {
				return record, err
			}
		}
		if record.Phase == "deleting" {
			if err := store.AdvanceWindowsScratchRecovery(ctx, record, "removed"); err != nil {
				return record, err
			}
			record.Phase = "removed"
			proof.Record = record
		}
		if record.Phase == "removed" {
			if err := store.AdvanceWindowsScratchRecovery(ctx, record, "complete"); err != nil {
				return record, err
			}
			record.Phase = "complete"
			proof.Record = record
		}
		return record, nil
	}
	if err := ctx.Err(); err != nil {
		return record, err
	}
	quarantine, err := root.OpenOrCreateRootNoSymlink("windows-quarantine", 0700)
	if err != nil {
		return record, err
	}
	defer quarantine.Close()
	quarantineID, err := quarantine.WindowsDirectoryIdentity()
	if err != nil {
		return record, err
	}
	record, err = store.PrepareWindowsScratchRecovery(ctx, key, quarantineID)
	if err != nil {
		return record, err
	}
	advance := func(next string) error {
		if err := store.AdvanceWindowsScratchRecovery(ctx, record, next); err != nil {
			return err
		}
		record.Phase = next
		if proof != nil && proof.Removed {
			proof.Record = record
		}
		return nil
	}
	if record.Phase == "complete" {
		return record, nil
	}
	if record.Phase == "removed" {
		return record, advance("complete")
	}
	var object *confinedfs.WindowsQuarantinedDirectory
	// Always inspect the same protected target first. A move may have completed
	// before its acknowledgement; an exact existing object resumes that intent.
	object, targetErr := quarantine.OpenWindowsQuarantinedDirectory(key.LaunchID, record.LeafIdentity)
	if object == nil && record.Phase == "prepared" {
		parent, openErr := openParent(record.ScratchRoot)
		if openErr != nil {
			return record, errors.Join(targetErr, openErr)
		}
		defer parent.Close()
		parentID, identityErr := parent.WindowsDirectoryIdentity()
		if identityErr != nil || parentID != record.BaseIdentity {
			return record, errors.Join(errors.New("Windows scratch parent replaced"), identityErr)
		}
		object, err = parent.QuarantineWindowsDirectory(filepath.Base(record.ScratchRoot), quarantine, key.LaunchID, record.LeafIdentity)
		if err != nil {
			return record, errors.Join(targetErr, err)
		}
	}
	if object == nil {
		return record, errors.Join(errors.New("original Windows quarantine object unavailable; deletion not proven"), targetErr)
	}
	defer object.Close()
	if record.Phase == "prepared" {
		if err = flushWindowsScratchRoots(root, record, openParent); err != nil {
			return record, err
		}
		if err = advance("quarantined"); err != nil {
			return record, err
		}
	}
	if record.Phase == "quarantined" {
		if err = advance("deleting"); err != nil {
			return record, err
		}
	}
	if record.Phase != "deleting" {
		return record, errors.New("invalid Windows scratch cleanup phase")
	}
	if err = object.Delete(ctx); err != nil {
		return record, err
	}
	if proof != nil {
		proof.Removed = true
		proof.Record = record
	}
	if err = flushWindowsScratchRoots(root, record, openParent); err != nil {
		return record, err
	}
	if err = advance("removed"); err != nil {
		return record, err
	}
	err = advance("complete")
	return record, err
}

func flushWindowsScratchRoots(root *confinedfs.Root, record protocol.WindowsScratchRecovery, openParent func(string) (*confinedfs.Root, error)) error {
	quarantine, err := root.OpenRootNoSymlink("windows-quarantine")
	if err != nil {
		return err
	}
	defer quarantine.Close()
	id, err := quarantine.WindowsDirectoryIdentity()
	if err != nil || id != record.QuarantineIdentity {
		return errors.Join(errors.New("Windows quarantine identity changed"), err)
	}
	parent, err := openParent(record.ScratchRoot)
	if err != nil {
		return err
	}
	defer parent.Close()
	id, err = parent.WindowsDirectoryIdentity()
	if err != nil || id != record.BaseIdentity {
		return errors.Join(errors.New("Windows scratch parent changed during flush"), err)
	}
	return errors.Join(parent.FlushWindowsDirectory(), quarantine.FlushWindowsDirectory())
}

// RecoverWindowsSandboxScratch requires both exclusive instance ownership and
// exact SDK cleaned. It does not recover or restart an unknown SDK execution.
func (m *Manager) RecoverWindowsSandboxScratch(ctx context.Context, key protocol.WindowsSandboxKey, ownership SandboxProcessRecoveryOwnership) (protocol.WindowsScratchRecovery, error) {
	var result protocol.WindowsScratchRecovery
	if ownership == nil {
		return result, errors.New("Windows scratch recovery requires exclusive ownership")
	}
	err := ownership.WithOwnership(func(appRoot string) error {
		gate, err := m.BeginClientStartup(ctx, key.SessionKey, key.OwnerUserID)
		if err != nil {
			return err
		}
		defer gate.Close()
		m.mu.RLock()
		config := m.windowsSandboxSupervisor
		store, ok := m.sandboxReceiptStore.(windowsScratchStore)
		state := m.sessions[key.SessionKey]
		active := state != nil && (state.Client != nil || state.Closing)
		m.mu.RUnlock()
		if active || config == nil || !ok {
			return errors.New("Windows scratch recovery requires inactive configured runtime")
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
		process, found, err := store.WindowsSandbox(ctx, key)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("original Windows launch missing")
		}
		if sandboxResourceIsActive(process.Intent.ScratchRoot) {
			return errors.New("Windows scratch still has original in-process owner")
		}
		statePath, err := filepath.EvalSymlinks(filepath.Dir(appRoot))
		if err != nil {
			return err
		}
		expectedBase := filepath.Join(statePath, "users", safeOwnerPathSegment(key.OwnerUserID), "runtime", scratchDirName)
		if filepath.Dir(process.Intent.ScratchRoot) != expectedBase {
			return errors.New("Windows scratch outside canonical owner runtime")
		}
		result, err = resumeWindowsScratchCleanup(ctx, config.Root, store, key, nil, func(path string) (*confinedfs.Root, error) {
			if filepath.Dir(path) != expectedBase {
				return nil, errors.New("Windows scratch scope changed")
			}
			stateRoot, err := confinedfs.Open(statePath)
			if err != nil {
				return nil, err
			}
			defer stateRoot.Close()
			return stateRoot.OpenRootNoSymlink(filepath.Join("users", safeOwnerPathSegment(key.OwnerUserID), "runtime", scratchDirName))
		})
		return err
	})
	return result, err
}
