// INPUT: Windows原launch/lease与完整cleaned回执，正常宿主删除后的确认。
// OUTPUT: scratch pending/complete持久责任；冷恢复不按路径不存在推断完成。
// POS: SDK资源撤销与Nexus宿主scratch删除是独立阶段。
package sandbox

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) WindowsSandboxResources(ctx context.Context, owner, session, lease string) (protocol.WindowsSandboxResources, bool, error) {
	var record protocol.WindowsSandboxResources
	if r == nil || r.db == nil || owner == "" || session == "" || lease == "" {
		return record, false, ErrWindowsSandboxConflict
	}
	b := r.dialect.Bind
	err := r.db.QueryRowContext(ctx, `SELECT launch_id,scratch_root,phase FROM windows_sandbox_resources WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3), owner, session, lease).Scan(&record.LaunchID, &record.ScratchRoot, &record.Phase)
	if errors.Is(err, sql.ErrNoRows) {
		return record, false, nil
	}
	if err != nil {
		return record, false, err
	}
	if !validWindowsSandboxNonce(record.LaunchID, 16) || record.ScratchRoot == "" || (record.Phase != "pending" && record.Phase != "complete") {
		return record, false, ErrWindowsSandboxConflict
	}
	record.OwnerUserID, record.SessionKey, record.LeaseID = owner, session, lease
	return record, true, nil
}

// CompleteWindowsSandboxResources 只由原live owner完成真实删除后调用；本函数不把路径缺失当证明。
func (r *Repository) CompleteWindowsSandboxResources(ctx context.Context, key protocol.WindowsSandboxKey) error {
	process, found, err := r.WindowsSandbox(ctx, key)
	if err != nil {
		return err
	}
	if !found || process.Phase != "cleaned" || process.Outcome == nil || !process.Outcome.Cleaned || process.Prepared == nil || process.Outcome.Prepared != *process.Prepared {
		return ErrWindowsSandboxConflict
	}
	record, found, err := r.WindowsSandboxResources(ctx, key.OwnerUserID, key.SessionKey, process.Intent.LeaseID)
	if err != nil {
		return err
	}
	if !found || record.LaunchID != key.LaunchID || record.ScratchRoot != process.Intent.ScratchRoot {
		return ErrWindowsSandboxConflict
	}
	if record.Phase == "complete" {
		return nil
	}
	b := r.dialect.Bind
	result, err := r.db.ExecContext(ctx, `UPDATE windows_sandbox_resources SET phase='complete' WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3)+` AND launch_id=`+b(4)+` AND scratch_root=`+b(5)+` AND phase='pending' AND NOT EXISTS(SELECT 1 FROM windows_sandbox_scopes WHERE owner_user_id=`+b(6)+` AND session_key=`+b(7)+` AND active_launch_id<>'')`, key.OwnerUserID, key.SessionKey, process.Intent.LeaseID, key.LaunchID, process.Intent.ScratchRoot, key.OwnerUserID, key.SessionKey)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrWindowsSandboxConflict
	}
	return nil
}

// PendingWindowsSandboxResources 保留已cleaned进程但scratch尚未证明删除的责任，不能仅扫描运行中launch。
func (r *Repository) PendingWindowsSandboxResources(ctx context.Context, owner, session string) (bool, error) {
	if r == nil || r.db == nil {
		return false, ErrWindowsSandboxConflict
	}
	b := r.dialect.Bind
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM windows_sandbox_resources WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND phase<>'complete'`, owner, session).Scan(&count)
	return count != 0, err
}

func (r *Repository) PendingWindowsSandboxResourceRecords(ctx context.Context, owner, session string) ([]protocol.WindowsSandboxResources, error) {
	if r == nil || r.db == nil || owner == "" || session == "" {
		return nil, ErrWindowsSandboxConflict
	}
	b := r.dialect.Bind
	rows, err := r.db.QueryContext(ctx, `SELECT lease_id,launch_id,scratch_root FROM windows_sandbox_resources WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND phase='pending'`, owner, session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []protocol.WindowsSandboxResources
	for rows.Next() {
		record := protocol.WindowsSandboxResources{OwnerUserID: owner, SessionKey: session, Phase: "pending"}
		if err := rows.Scan(&record.LeaseID, &record.LaunchID, &record.ScratchRoot); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
