// INPUT: 已收口（reaped/aborted）原进程 key 与 owner 显式放弃理由。
// OUTPUT: 资源/策略栅栏在一个事务内收口；原 unknown 理由与放弃理由一并保留。
// POS: 启动恢复失败后的 owner 显式出口；不访问文件系统，不重放任务，也不放行仍可能存活的进程。
package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const maxDiscardReasonBytes = 512

// DiscardProcessLifecycle closes the resource and policy fences of a terminal
// launch whose automatic recovery cannot finish (for example a scratch
// directory the owner already deleted). It is an explicit owner decision, not
// evidence that cleanup happened: the scratch recovery row is marked complete,
// the launch resource phase is completed, and bound receipts become reconciled
// with the reason appended to their unknown_reason for audit. Launches that are
// still prepared/registered/released are rejected because their process may be
// alive.
func (r *Repository) DiscardProcessLifecycle(ctx context.Context, key protocol.SandboxProcessKey, reason string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("sandbox lifecycle repository unavailable")
	}
	if !validProcessKey(key) {
		return 0, ErrInvalidProcess
	}
	reason = boundedDiscardReason(reason)
	if reason == "" {
		return 0, errors.New("sandbox lifecycle discard requires a reason")
	}
	process, found, err := r.Process(ctx, key)
	if err != nil {
		return 0, err
	}
	if !found || (process.Phase != protocol.SandboxProcessReaped && process.Phase != protocol.SandboxProcessAborted) {
		return 0, ErrProcessConflict
	}
	b := r.dialect.Bind
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if process.Intent.LeaseID != "" {
		data, err := json.Marshal(process.Intent.Scratch)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sandbox_scratch_recoveries(owner_user_id,session_key,lease_id,source_generation,source_launch_id,scratch_json,phase) VALUES(`+b(1)+`,`+b(2)+`,`+b(3)+`,`+b(4)+`,`+b(5)+`,`+b(6)+`,'complete') ON CONFLICT DO NOTHING`, key.OwnerUserID, key.SessionKey, process.Intent.LeaseID, key.Generation, key.LaunchID, string(data)); err != nil {
			return 0, err
		}
		// A probe of the same lease may have created the row first; it keeps its
		// original source binding, but must describe this launch's exact scratch.
		result, err := tx.ExecContext(ctx, `UPDATE sandbox_scratch_recoveries SET phase='complete' WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3)+` AND scratch_json=`+b(4), key.OwnerUserID, key.SessionKey, process.Intent.LeaseID, string(data))
		if err != nil {
			return 0, err
		}
		if rows, err := result.RowsAffected(); err != nil {
			return 0, err
		} else if rows != 1 {
			return 0, ErrScratchRecoveryConflict
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sandbox_process_launches SET resource_phase='complete' WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND generation=`+b(3)+` AND launch_id=`+b(4), key.OwnerUserID, key.SessionKey, key.Generation, key.LaunchID); err != nil {
		return 0, err
	}
	note := "discarded by owner: " + reason
	result, err := tx.ExecContext(ctx, `UPDATE sandbox_policy_receipts SET phase='reconciled',unknown_reason=CASE WHEN unknown_reason='' THEN `+b(1)+` ELSE unknown_reason || '; ' || `+b(2)+` END,updated_at=`+b(3)+` WHERE owner_user_id=`+b(4)+` AND session_key=`+b(5)+` AND process_launch_id=`+b(6)+` AND phase IN ('confirmed','retiring','unknown')`, note, note, r.dialect.TimestampValue(time.Now().UTC()), key.OwnerUserID, key.SessionKey, key.LaunchID)
	if err != nil {
		return 0, err
	}
	receipts, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return receipts, nil
}

func boundedDiscardReason(reason string) string {
	reason = strings.Join(strings.Fields(reason), " ")
	if len(reason) <= maxDiscardReasonBytes {
		return reason
	}
	cut := maxDiscardReasonBytes
	for cut > 0 && !utf8.RuneStart(reason[cut]) {
		cut--
	}
	return reason[:cut]
}
