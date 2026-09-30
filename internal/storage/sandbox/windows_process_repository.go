// INPUT: 产品固定 Windows scope、不可复用启动意图及 Bridge 认证的双摘要/清理回执。
// OUTPUT: 独立 Windows pending 栅栏、一次性 start CAS 和不会被新代次绕过的 unknown。
// POS: 只保存摘要与事实，不保存任务命令、环境或账号密码，不自动恢复执行。
package sandbox

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ReserveWindowsSandbox 在创建机器 Host 前占据原始会话 scope；用途顺序与代次只增不退。
func (r *Repository) ReserveWindowsSandbox(ctx context.Context, intent protocol.WindowsSandboxIntent) error {
	if r == nil || r.db == nil {
		return ErrWindowsSandboxConflict
	}
	if err := validateWindowsSandboxIntent(intent); err != nil {
		return err
	}
	body, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	key := intent.Key
	order, _ := protocol.WindowsSandboxPurposeOrder(intent.Purpose)
	b := r.dialect.Bind
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO windows_sandbox_scopes(owner_user_id,session_key) VALUES(`+b(1)+`,`+b(2)+`) ON CONFLICT DO NOTHING`, key.OwnerUserID, key.SessionKey); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE windows_sandbox_scopes SET generation=`+b(1)+`,launch_order=`+b(2)+`,active_launch_id=`+b(3)+` WHERE owner_user_id=`+b(4)+` AND session_key=`+b(5)+` AND active_launch_id='' AND (generation<`+b(6)+` OR (generation=`+b(7)+` AND launch_order<`+b(8)+`)) AND NOT EXISTS(SELECT 1 FROM sandbox_scratch_recoveries WHERE owner_user_id=`+b(9)+` AND session_key=`+b(10)+` AND phase<>'complete')`, key.Generation, order, key.LaunchID, key.OwnerUserID, key.SessionKey, key.Generation, key.Generation, order, key.OwnerUserID, key.SessionKey)
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
	_, err = tx.ExecContext(ctx, `INSERT INTO windows_sandbox_launches(owner_user_id,session_key,generation,launch_id,launch_order,phase,intent_json) VALUES(`+b(1)+`,`+b(2)+`,`+b(3)+`,`+b(4)+`,`+b(5)+`,'reserved',`+b(6)+`)`, key.OwnerUserID, key.SessionKey, key.Generation, key.LaunchID, order, string(body))
	if err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO windows_sandbox_resources(owner_user_id,session_key,lease_id,launch_id,scratch_root,phase) VALUES(`+b(1)+`,`+b(2)+`,`+b(3)+`,`+b(4)+`,`+b(5)+`,'pending') ON CONFLICT(owner_user_id,session_key,lease_id) DO UPDATE SET launch_id=excluded.launch_id WHERE windows_sandbox_resources.phase='pending' AND windows_sandbox_resources.scratch_root=excluded.scratch_root`, key.OwnerUserID, key.SessionKey, intent.LeaseID, key.LaunchID, intent.ScratchRoot)
	if err != nil {
		return err
	}
	count, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrWindowsSandboxConflict
	}
	return tx.Commit()
}

// RecordWindowsSandboxPrepared 只为同一 reserved 意图保存 SDK 实际执行身份；旧 start 后不能重新准备。
func (r *Repository) RecordWindowsSandboxPrepared(ctx context.Context, intent protocol.WindowsSandboxIntent, prepared protocol.WindowsSandboxPrepared) error {
	snapshot, found, err := r.WindowsSandbox(ctx, intent.Key)
	if err != nil {
		return err
	}
	if !found || snapshot.Intent != intent || !validWindowsSandboxPrepared(prepared) {
		return ErrWindowsSandboxConflict
	}
	if snapshot.Phase == "prepared" && snapshot.Prepared != nil && *snapshot.Prepared == prepared {
		return nil
	}
	if snapshot.Phase != "reserved" {
		return ErrWindowsSandboxConflict
	}
	next := snapshot
	next.Phase, next.Prepared = "prepared", &prepared
	return r.transitionWindowsSandbox(ctx, snapshot, next)
}

// ClaimWindowsSandboxStart 是单次 CAS；响应未知也不提供幂等成功回放。
func (r *Repository) ClaimWindowsSandboxStart(ctx context.Context, intent protocol.WindowsSandboxIntent, prepared protocol.WindowsSandboxPrepared) error {
	snapshot, found, err := r.WindowsSandbox(ctx, intent.Key)
	if err != nil {
		return err
	}
	if !found || snapshot.Intent != intent || snapshot.Phase != "prepared" || snapshot.Prepared == nil || *snapshot.Prepared != prepared {
		return ErrWindowsSandboxConflict
	}
	next := snapshot
	next.Phase = "started"
	return r.transitionWindowsSandbox(ctx, snapshot, next)
}

// FinishWindowsSandbox unknown 保留 pending 唯一槽；仅匹配实际 prepared 的完整 cleaned 才释放。
func (r *Repository) FinishWindowsSandbox(ctx context.Context, intent protocol.WindowsSandboxIntent, outcome protocol.WindowsSandboxOutcome) error {
	snapshot, found, err := r.WindowsSandbox(ctx, intent.Key)
	if err != nil {
		return err
	}
	if !found || snapshot.Intent != intent {
		return ErrWindowsSandboxConflict
	}
	if snapshot.Phase == "cleaned" {
		if snapshot.Outcome != nil && *snapshot.Outcome == outcome {
			return nil
		}
		return ErrWindowsSandboxConflict
	}
	if snapshot.Prepared == nil {
		if outcome.Cleaned {
			return ErrWindowsSandboxConflict
		}
		if outcome.Prepared != (protocol.WindowsSandboxPrepared{}) {
			if !validWindowsSandboxPrepared(outcome.Prepared) {
				return ErrWindowsSandboxConflict
			}
		}
	} else if outcome.Prepared != *snapshot.Prepared {
		return ErrWindowsSandboxConflict
	}
	next := snapshot
	if next.Prepared == nil && outcome.Prepared != (protocol.WindowsSandboxPrepared{}) {
		prepared := outcome.Prepared
		next.Prepared = &prepared
	}
	next.Phase, next.Outcome = "unknown", &outcome
	if outcome.Cleaned {
		next.Phase = "cleaned"
	}
	if err := validateWindowsSandboxSnapshot(next); err != nil {
		return err
	}
	if snapshot.Phase == next.Phase && snapshot.Outcome != nil && *snapshot.Outcome == outcome {
		return nil
	}
	return r.transitionWindowsSandbox(ctx, snapshot, next)
}

// transitionWindowsSandbox 同时比较阶段和原登记正文，拒绝并发 cleanup/start 或不同回执覆盖。
func (r *Repository) transitionWindowsSandbox(ctx context.Context, before, next protocol.WindowsSandboxSnapshot) error {
	if err := validateWindowsSandboxSnapshot(next); err != nil {
		return err
	}
	encode := func(value any) string { body, _ := json.Marshal(value); return string(body) }
	optionalPrepared := func(p *protocol.WindowsSandboxPrepared) string {
		if p == nil {
			return ""
		}
		return encode(p)
	}
	optionalOutcome := func(p *protocol.WindowsSandboxOutcome) string {
		if p == nil {
			return ""
		}
		return encode(p)
	}
	key, b := before.Intent.Key, r.dialect.Bind
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE windows_sandbox_launches SET phase=`+b(1)+`,prepared_json=`+b(2)+`,outcome_json=`+b(3)+` WHERE owner_user_id=`+b(4)+` AND session_key=`+b(5)+` AND generation=`+b(6)+` AND launch_id=`+b(7)+` AND phase=`+b(8)+` AND intent_json=`+b(9)+` AND prepared_json=`+b(10)+` AND outcome_json=`+b(11), next.Phase, optionalPrepared(next.Prepared), optionalOutcome(next.Outcome), key.OwnerUserID, key.SessionKey, key.Generation, key.LaunchID, before.Phase, encode(before.Intent), optionalPrepared(before.Prepared), optionalOutcome(before.Outcome))
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
	if next.Phase == "cleaned" {
		order, _ := protocol.WindowsSandboxPurposeOrder(next.Intent.Purpose)
		result, err = tx.ExecContext(ctx, `UPDATE windows_sandbox_scopes SET active_launch_id='' WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND generation=`+b(3)+` AND launch_order=`+b(4)+` AND active_launch_id=`+b(5), key.OwnerUserID, key.SessionKey, key.Generation, order, key.LaunchID)
		if err != nil {
			return err
		}
		count, err = result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrWindowsSandboxConflict
		}
	}
	return tx.Commit()
}

func (r *Repository) WindowsSandbox(ctx context.Context, key protocol.WindowsSandboxKey) (protocol.WindowsSandboxSnapshot, bool, error) {
	if r == nil || r.db == nil {
		return protocol.WindowsSandboxSnapshot{}, false, ErrWindowsSandboxConflict
	}
	if err := validateWindowsSandboxKey(key); err != nil {
		return protocol.WindowsSandboxSnapshot{}, false, err
	}
	b := r.dialect.Bind
	return r.readWindowsSandbox(ctx, `WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND generation=`+b(3)+` AND launch_id=`+b(4), key.OwnerUserID, key.SessionKey, key.Generation, key.LaunchID)
}

// LatestWindowsSandbox 读取原始 scope 的最高代次/用途，unknown 不能改 session 后缀规避。
func (r *Repository) LatestWindowsSandbox(ctx context.Context, owner, session string) (protocol.WindowsSandboxSnapshot, bool, error) {
	if r == nil || r.db == nil {
		return protocol.WindowsSandboxSnapshot{}, false, ErrWindowsSandboxConflict
	}
	if owner == "" || session == "" {
		return protocol.WindowsSandboxSnapshot{}, false, ErrWindowsSandboxConflict
	}
	b := r.dialect.Bind
	return r.readWindowsSandbox(ctx, `WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` ORDER BY generation DESC,launch_order DESC LIMIT 1`, owner, session)
}

func (r *Repository) readWindowsSandbox(ctx context.Context, where string, args ...any) (protocol.WindowsSandboxSnapshot, bool, error) {
	var snapshot protocol.WindowsSandboxSnapshot
	if r == nil || r.db == nil {
		return snapshot, false, ErrWindowsSandboxConflict
	}
	var key protocol.WindowsSandboxKey
	var generation int64
	var order int
	var intent, prepared, outcome string
	err := r.db.QueryRowContext(ctx, `SELECT owner_user_id,session_key,generation,launch_id,launch_order,phase,intent_json,prepared_json,outcome_json FROM windows_sandbox_launches `+where, args...).Scan(&key.OwnerUserID, &key.SessionKey, &generation, &key.LaunchID, &order, &snapshot.Phase, &intent, &prepared, &outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return snapshot, false, nil
	}
	if err != nil {
		return snapshot, false, err
	}
	key.Generation = uint64(generation)
	if generation <= 0 || json.Unmarshal([]byte(intent), &snapshot.Intent) != nil || snapshot.Intent.Key != key {
		return snapshot, false, ErrWindowsSandboxConflict
	}
	expected, ok := protocol.WindowsSandboxPurposeOrder(snapshot.Intent.Purpose)
	if !ok || order != expected {
		return snapshot, false, ErrWindowsSandboxConflict
	}
	if prepared != "" {
		if json.Unmarshal([]byte(prepared), &snapshot.Prepared) != nil || snapshot.Prepared == nil {
			return snapshot, false, ErrWindowsSandboxConflict
		}
	}
	if outcome != "" {
		if json.Unmarshal([]byte(outcome), &snapshot.Outcome) != nil || snapshot.Outcome == nil {
			return snapshot, false, ErrWindowsSandboxConflict
		}
	}
	if err := validateWindowsSandboxSnapshot(snapshot); err != nil {
		return snapshot, false, err
	}
	for _, item := range []struct {
		body  string
		value any
	}{{intent, snapshot.Intent}, {prepared, snapshot.Prepared}, {outcome, snapshot.Outcome}} {
		if item.body == "" {
			continue
		}
		canonical, err := json.Marshal(item.value)
		if err != nil || !bytes.Equal(canonical, []byte(item.body)) {
			return snapshot, false, ErrWindowsSandboxConflict
		}
	}
	return snapshot, true, nil
}

// PendingWindowsSandboxKeys 供持有实例所有权的宿主列出保留责任；分页游标只读，不自动重放或清理。
func (r *Repository) PendingWindowsSandboxKeys(ctx context.Context, after string, limit int) ([]protocol.WindowsSandboxKey, bool, error) {
	if r == nil || r.db == nil || limit < 1 || limit > 256 || (after != "" && !validWindowsSandboxNonce(after, 16)) {
		return nil, false, ErrWindowsSandboxConflict
	}
	b := r.dialect.Bind
	rows, err := r.db.QueryContext(ctx, `SELECT owner_user_id,session_key,generation,launch_id FROM windows_sandbox_launches WHERE phase<>'cleaned' AND launch_id>`+b(1)+` ORDER BY launch_id LIMIT `+b(2), after, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	keys := make([]protocol.WindowsSandboxKey, 0, limit)
	for rows.Next() {
		var key protocol.WindowsSandboxKey
		var generation int64
		if err := rows.Scan(&key.OwnerUserID, &key.SessionKey, &generation, &key.LaunchID); err != nil {
			return nil, false, err
		}
		if generation <= 0 {
			return nil, false, ErrWindowsSandboxConflict
		}
		key.Generation = uint64(generation)
		if err := validateWindowsSandboxKey(key); err != nil {
			return nil, false, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(keys) > limit
	if more {
		keys = keys[:limit]
	}
	return keys, more, nil
}
