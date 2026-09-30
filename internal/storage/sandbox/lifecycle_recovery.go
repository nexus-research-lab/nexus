// INPUT: 原进程 key、资源完成事实及有界恢复游标。
// OUTPUT: 进程已收口之后的资源/策略待办发现与资源阶段确认。
// POS: 启动恢复扫描；不会因 native pending 扫描为空而漏掉后续清理。
package sandbox

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) CompleteProcessResources(ctx context.Context, key protocol.SandboxProcessKey) error {
	process, found, err := r.Process(ctx, key)
	if err != nil {
		return err
	}
	if !found || (process.Phase != protocol.SandboxProcessReaped && process.Phase != protocol.SandboxProcessAborted) {
		return ErrProcessConflict
	}
	if process.Intent.LeaseID != "" {
		recovery, found, err := r.ScratchRecovery(ctx, key.OwnerUserID, key.SessionKey, process.Intent.LeaseID)
		if err != nil {
			return err
		}
		if !found || recovery.Phase != protocol.SandboxScratchRecoveryComplete || recovery.Scratch != process.Intent.Scratch {
			return ErrScratchRecoveryConflict
		}
	}
	b := r.dialect.Bind
	_, err = r.db.ExecContext(ctx, `UPDATE sandbox_process_launches SET resource_phase='complete' WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND generation=`+b(3)+` AND launch_id=`+b(4), key.OwnerUserID, key.SessionKey, key.Generation, key.LaunchID)
	return err
}

// PendingLifecycleKeys includes terminal processes with unfinished resource
// work or explicitly bound policy receipts, even when native pending is empty.
func (r *Repository) PendingLifecycleKeys(ctx context.Context, after string, limit int) ([]protocol.SandboxProcessKey, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("sandbox lifecycle repository unavailable")
	}
	if limit < 1 || limit > 256 || (after != "" && !validLowerHex(after, 32)) {
		return nil, false, ErrInvalidProcess
	}
	b := r.dialect.Bind
	rows, err := r.db.QueryContext(ctx, `SELECT p.owner_user_id,p.session_key,p.generation,p.launch_id FROM sandbox_process_launches p WHERE p.phase IN ('aborted','reaped') AND p.launch_id IN (SELECT launch_id FROM sandbox_process_launches WHERE resource_phase='pending' UNION SELECT process_launch_id FROM sandbox_policy_receipts WHERE process_launch_id<>'' AND phase IN ('confirmed','retiring','unknown')) AND p.launch_id>`+b(1)+` ORDER BY p.launch_id LIMIT `+b(2), after, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	keys := make([]protocol.SandboxProcessKey, 0, limit)
	for rows.Next() {
		var key protocol.SandboxProcessKey
		var generation int64
		if err := rows.Scan(&key.OwnerUserID, &key.SessionKey, &generation, &key.LaunchID); err != nil {
			return nil, false, err
		}
		if generation <= 0 {
			return nil, false, ErrInvalidProcess
		}
		key.Generation = uint64(generation)
		if !validProcessKey(key) || key.LaunchID <= after {
			return nil, false, ErrInvalidProcess
		}
		if len(keys) == limit {
			return keys, true, nil
		}
		keys = append(keys, key)
		after = key.LaunchID
	}
	return keys, false, rows.Err()
}
