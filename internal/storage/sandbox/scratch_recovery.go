// INPUT: 已收口原进程、可信目录身份与 exact 清理阶段。
// OUTPUT: 不可改绑的清理记录和 pending 启动栅栏。
// POS: 回收区文件动作之前/之后的持久边界，不凭路径缺失判定成功。
package sandbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var ErrScratchRecoveryConflict = errors.New("sandbox scratch recovery conflict")

func (r *Repository) PrepareScratchRecovery(ctx context.Context, key protocol.SandboxProcessKey) (protocol.SandboxScratchRecovery, error) {
	empty := protocol.SandboxScratchRecovery{}
	process, found, err := r.Process(ctx, key)
	if err != nil {
		return empty, err
	}
	if !found || (process.Phase != protocol.SandboxProcessReaped && process.Phase != protocol.SandboxProcessAborted) || process.Intent.Scratch == (protocol.SandboxProcessScratch{}) || !validLowerHex(process.Intent.LeaseID, 32) {
		return empty, ErrScratchRecoveryConflict
	}
	data, err := json.Marshal(process.Intent.Scratch)
	if err != nil {
		return empty, err
	}
	b := r.dialect.Bind
	_, err = r.db.ExecContext(ctx, `INSERT INTO sandbox_scratch_recoveries(owner_user_id,session_key,lease_id,source_generation,source_launch_id,scratch_json,phase) SELECT `+b(1)+`,`+b(2)+`,`+b(3)+`,`+b(4)+`,`+b(5)+`,`+b(6)+`,'prepared' WHERE NOT EXISTS(SELECT 1 FROM sandbox_process_launches WHERE owner_user_id=`+b(7)+` AND session_key=`+b(8)+` AND phase IN ('prepared','registered','released')) ON CONFLICT DO NOTHING`, key.OwnerUserID, key.SessionKey, process.Intent.LeaseID, key.Generation, key.LaunchID, string(data), key.OwnerUserID, key.SessionKey)
	if err != nil {
		return empty, err
	}
	result, found, err := r.ScratchRecovery(ctx, key.OwnerUserID, key.SessionKey, process.Intent.LeaseID)
	if err != nil {
		return empty, err
	}
	if !found || result.Scratch != process.Intent.Scratch {
		return empty, ErrScratchRecoveryConflict
	}
	// A later runtime may share a probe's resource. Reuse only its immutable
	// resource record, and reject any currently live launch before filesystem IO.
	var active int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sandbox_process_launches WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND phase IN ('prepared','registered','released')`, key.OwnerUserID, key.SessionKey).Scan(&active)
	if err != nil {
		return empty, err
	}
	if active != 0 {
		return empty, ErrScratchRecoveryConflict
	}
	return result, nil
}

func (r *Repository) ScratchRecovery(ctx context.Context, owner, session, lease string) (protocol.SandboxScratchRecovery, bool, error) {
	result := protocol.SandboxScratchRecovery{}
	if r == nil || r.db == nil || owner == "" || session == "" || !validLowerHex(lease, 32) {
		return result, false, ErrScratchRecoveryConflict
	}
	b := r.dialect.Bind
	var generation int64
	var encoded string
	err := r.db.QueryRowContext(ctx, `SELECT source_generation,source_launch_id,scratch_json,phase FROM sandbox_scratch_recoveries WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3), owner, session, lease).Scan(&generation, &result.ProcessKey.LaunchID, &encoded, &result.Phase)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if generation <= 0 || json.Unmarshal([]byte(encoded), &result.Scratch) != nil || result.Scratch == (protocol.SandboxProcessScratch{}) {
		return protocol.SandboxScratchRecovery{}, false, ErrScratchRecoveryConflict
	}
	result.ProcessKey.OwnerUserID = owner
	result.ProcessKey.SessionKey = session
	result.ProcessKey.Generation = uint64(generation)
	result.LeaseID = lease
	if !validProcessKey(result.ProcessKey) || !validScratchRecoveryPhase(result.Phase) {
		return protocol.SandboxScratchRecovery{}, false, ErrScratchRecoveryConflict
	}
	source, found, err := r.Process(ctx, result.ProcessKey)
	if err != nil {
		return protocol.SandboxScratchRecovery{}, false, err
	}
	if !found || source.Intent.LeaseID != lease || source.Intent.Scratch != result.Scratch || (source.Phase != protocol.SandboxProcessReaped && source.Phase != protocol.SandboxProcessAborted) {
		return protocol.SandboxScratchRecovery{}, false, ErrScratchRecoveryConflict
	}
	return result, true, nil
}

func validScratchRecoveryPhase(phase protocol.SandboxScratchRecoveryPhase) bool {
	switch phase {
	case protocol.SandboxScratchRecoveryPrepared, protocol.SandboxScratchRecoveryQuarantined, protocol.SandboxScratchRecoveryDeleting, protocol.SandboxScratchRecoveryComplete:
		return true
	}
	return false
}

func (r *Repository) AdvanceScratchRecovery(ctx context.Context, expected protocol.SandboxScratchRecovery, next protocol.SandboxScratchRecoveryPhase) error {
	allowed := expected.Phase == protocol.SandboxScratchRecoveryPrepared && next == protocol.SandboxScratchRecoveryQuarantined || expected.Phase == protocol.SandboxScratchRecoveryQuarantined && next == protocol.SandboxScratchRecoveryDeleting || expected.Phase == protocol.SandboxScratchRecoveryDeleting && next == protocol.SandboxScratchRecoveryComplete
	if !allowed {
		return ErrScratchRecoveryConflict
	}
	current, found, err := r.ScratchRecovery(ctx, expected.ProcessKey.OwnerUserID, expected.ProcessKey.SessionKey, expected.LeaseID)
	if err != nil {
		return err
	}
	if !found || current.ProcessKey != expected.ProcessKey || current.Scratch != expected.Scratch || (current.Phase != expected.Phase && current.Phase != next) {
		return ErrScratchRecoveryConflict
	}
	b := r.dialect.Bind
	_, err = r.db.ExecContext(ctx, `UPDATE sandbox_scratch_recoveries SET phase=`+b(1)+` WHERE owner_user_id=`+b(2)+` AND session_key=`+b(3)+` AND lease_id=`+b(4)+` AND source_launch_id=`+b(5)+` AND phase=`+b(6), next, expected.ProcessKey.OwnerUserID, expected.ProcessKey.SessionKey, expected.LeaseID, expected.ProcessKey.LaunchID, expected.Phase)
	if err != nil {
		return err
	}
	current, found, err = r.ScratchRecovery(ctx, expected.ProcessKey.OwnerUserID, expected.ProcessKey.SessionKey, expected.LeaseID)
	if err != nil {
		return err
	}
	if !found || current.Phase != next {
		return ErrScratchRecoveryConflict
	}
	return nil
}

func (r *Repository) PendingScratchRecovery(ctx context.Context, owner, session string) (bool, error) {
	if r == nil || r.db == nil || owner == "" || session == "" {
		return false, ErrScratchRecoveryConflict
	}
	b := r.dialect.Bind
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sandbox_scratch_recoveries WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND phase<>'complete'`, owner, session).Scan(&count)
	return count != 0, err
}
