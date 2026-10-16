// INPUT: SDK完整cleaned、原启动目录身份与固定回收区身份。
// OUTPUT: 单调持久回收阶段；最终资源complete与删除证明同事务。
// POS: 仅精确资源恢复，不按缺失路径或进程退出清理unknown。
package sandbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"strings"
)

func (r *Repository) PrepareWindowsScratchRecovery(ctx context.Context, key protocol.WindowsSandboxKey, quarantineID string) (protocol.WindowsScratchRecovery, error) {
	var empty protocol.WindowsScratchRecovery
	p, found, err := r.WindowsSandbox(ctx, key)
	if err != nil {
		return empty, err
	}
	if !found || p.Phase != "cleaned" || p.Prepared == nil || p.Outcome == nil || !p.Outcome.Cleaned || p.Outcome.Prepared != *p.Prepared || !strings.HasPrefix(p.Intent.ScratchBaseIdentity, "windows-v1:") || !strings.HasPrefix(p.Intent.ScratchLeafIdentity, "windows-v1:") || !strings.HasPrefix(quarantineID, "windows-v1:") {
		return empty, ErrWindowsSandboxConflict
	}
	resources, found, err := r.WindowsSandboxResources(ctx, key.OwnerUserID, key.SessionKey, p.Intent.LeaseID)
	if err != nil {
		return empty, err
	}
	if !found || resources.LaunchID != key.LaunchID || resources.ScratchRoot != p.Intent.ScratchRoot {
		return empty, ErrWindowsSandboxConflict
	}
	record := protocol.WindowsScratchRecovery{Key: key, LeaseID: p.Intent.LeaseID, ScratchRoot: p.Intent.ScratchRoot, BaseIdentity: p.Intent.ScratchBaseIdentity, LeafIdentity: p.Intent.ScratchLeafIdentity, QuarantineIdentity: quarantineID, Phase: "prepared"}
	body, err := json.Marshal(record)
	if err != nil {
		return empty, err
	}
	b := r.dialect.Bind
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	locked, err := tx.ExecContext(ctx, `UPDATE windows_sandbox_scopes SET active_launch_id=active_launch_id WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND active_launch_id=''`, key.OwnerUserID, key.SessionKey)
	if err != nil {
		return empty, err
	}
	if err = windowsScratchAffected(locked); err != nil {
		return empty, err
	}
	var actualLaunch, actualPath string
	if err = tx.QueryRowContext(ctx, `SELECT launch_id,scratch_root FROM windows_sandbox_resources WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3), key.OwnerUserID, key.SessionKey, record.LeaseID).Scan(&actualLaunch, &actualPath); err != nil {
		return empty, err
	}
	if actualLaunch != key.LaunchID || actualPath != record.ScratchRoot {
		return empty, ErrWindowsSandboxConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO windows_scratch_recoveries(owner_user_id,session_key,lease_id,launch_id,phase,record_json) VALUES(`+b(1)+`,`+b(2)+`,`+b(3)+`,`+b(4)+`,'prepared',`+b(5)+`) ON CONFLICT DO NOTHING`, key.OwnerUserID, key.SessionKey, record.LeaseID, key.LaunchID, string(body))
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	var actualBody, phase string
	err = r.db.QueryRowContext(ctx, `SELECT phase,record_json FROM windows_scratch_recoveries WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3)+` AND launch_id=`+b(4), key.OwnerUserID, key.SessionKey, record.LeaseID, key.LaunchID).Scan(&phase, &actualBody)
	if err != nil {
		return empty, err
	}
	var actual protocol.WindowsScratchRecovery
	if err = json.Unmarshal([]byte(actualBody), &actual); err != nil {
		return empty, err
	}
	canonical, err := json.Marshal(actual)
	if err != nil || string(canonical) != actualBody {
		return empty, ErrWindowsSandboxConflict
	}
	record.Phase = phase
	if actual != record || !windowsScratchPhase(phase) {
		return empty, ErrWindowsSandboxConflict
	}
	return actual, nil
}

func windowsScratchPhase(phase string) bool {
	switch phase {
	case "prepared", "quarantined", "deleting", "removed", "complete":
		return true
	}
	return false
}

func (r *Repository) AdvanceWindowsScratchRecovery(ctx context.Context, record protocol.WindowsScratchRecovery, next string) error {
	allowed := map[string]string{"prepared": "quarantined", "quarantined": "deleting", "deleting": "removed", "removed": "complete"}
	if allowed[record.Phase] != next {
		return ErrWindowsSandboxConflict
	}
	previous, err := json.Marshal(record)
	if err != nil {
		return err
	}
	updated := record
	updated.Phase = next
	body, err := json.Marshal(updated)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	b := r.dialect.Bind
	// Lock the scope cursor too: a new launch cannot race cleanup completion.
	result, err := tx.ExecContext(ctx, `UPDATE windows_sandbox_scopes SET active_launch_id=active_launch_id WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND active_launch_id=''`, record.Key.OwnerUserID, record.Key.SessionKey)
	if err != nil {
		return err
	}
	if err = windowsScratchAffected(result); err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `UPDATE windows_scratch_recoveries SET phase=`+b(1)+`,record_json=`+b(2)+` WHERE owner_user_id=`+b(3)+` AND session_key=`+b(4)+` AND lease_id=`+b(5)+` AND launch_id=`+b(6)+` AND phase=`+b(7)+` AND record_json=`+b(8), next, string(body), record.Key.OwnerUserID, record.Key.SessionKey, record.LeaseID, record.Key.LaunchID, record.Phase, string(previous))
	if err != nil {
		return err
	}
	if err = windowsScratchAffected(result); err != nil {
		var actual string
		readErr := tx.QueryRowContext(ctx, `SELECT record_json FROM windows_scratch_recoveries WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3)+` AND launch_id=`+b(4)+` AND phase=`+b(5), record.Key.OwnerUserID, record.Key.SessionKey, record.LeaseID, record.Key.LaunchID, next).Scan(&actual)
		if readErr != nil || actual != string(body) {
			return errors.Join(err, readErr)
		}
		// The complete transition and resource completion were atomic.
		return tx.Commit()
	}
	if next == "complete" {
		result, err = tx.ExecContext(ctx, `UPDATE windows_sandbox_resources SET phase='complete' WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND lease_id=`+b(3)+` AND launch_id=`+b(4)+` AND scratch_root=`+b(5)+` AND phase='pending'`, record.Key.OwnerUserID, record.Key.SessionKey, record.LeaseID, record.Key.LaunchID, record.ScratchRoot)
		if err != nil {
			return err
		}
		if err = windowsScratchAffected(result); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func windowsScratchAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.Join(ErrWindowsSandboxConflict, errors.New("Windows scratch transition did not own exact row"))
	}
	return nil
}
