// INPUT: exact 宿主启动身份、已认证集合登记与不可重放的一次放行。
// OUTPUT: 数据库唯一 active slot、执行前登记、单次 release CAS 与回收后终态。
// POS: 进程启动持久事实，与连接后策略回执共享 scope，但不混淆两种事实。
package sandbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// PrepareProcess 必须先于 launchd bootstrap。相同 prepared 意图可重读，不能重启已放行任务。
func (r *Repository) PrepareProcess(ctx context.Context, intent protocol.SandboxProcessIntent) error {
	if r == nil || r.db == nil {
		return errors.New("sandbox process repository is unavailable")
	}
	if err := validateProcessIntent(intent); err != nil {
		return err
	}
	data, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	b := r.dialect.Bind
	now := time.Now().UTC()
	k := intent.Key
	query := `INSERT INTO sandbox_process_launches(owner_user_id,session_key,generation,launch_id,phase,intent_json,created_at,updated_at)
 SELECT ` + b(1) + `,` + b(2) + `,` + b(3) + `,` + b(4) + `,'prepared',` + b(5) + `,` + b(6) + `,` + b(7) + `
 WHERE NOT EXISTS(SELECT 1 FROM sandbox_process_launches WHERE owner_user_id=` + b(8) + ` AND session_key=` + b(9) + ` AND generation>=` + b(10) + `) ON CONFLICT DO NOTHING`
	if _, err := r.db.ExecContext(ctx, query, k.OwnerUserID, k.SessionKey, k.Generation, k.LaunchID, string(data), now, now, k.OwnerUserID, k.SessionKey, k.Generation); err != nil {
		return err
	}
	got, found, err := r.Process(ctx, k)
	if err != nil {
		return err
	}
	if !found || got.Intent != intent || got.Phase != protocol.SandboxProcessPrepared {
		return ErrProcessConflict
	}
	return nil
}

// RegisterProcess 写入原集合；同一 registered 身份重试可幂等，但已放行身份不可再登记。
func (r *Repository) RegisterProcess(ctx context.Context, key protocol.SandboxProcessKey, registration protocol.SandboxProcessRegistration) error {
	snapshot, found, err := r.Process(ctx, key)
	if err != nil {
		return err
	}
	if !found {
		return ErrProcessConflict
	}
	if err := validateProcessRegistration(snapshot.Intent, registration); err != nil {
		return err
	}
	if snapshot.Phase == protocol.SandboxProcessRegistered && snapshot.Registration != nil && *snapshot.Registration == registration {
		return nil
	}
	if snapshot.Phase != protocol.SandboxProcessPrepared {
		return ErrProcessConflict
	}
	data, _ := json.Marshal(registration)
	return r.transitionProcess(ctx, key, protocol.SandboxProcessPrepared, protocol.SandboxProcessRegistered, string(data), "")
}

// ClaimProcessRelease 是一次性领取，不提供幂等成功回放。即使响应丢失也不得重发任务。
func (r *Repository) ClaimProcessRelease(ctx context.Context, key protocol.SandboxProcessKey) error {
	snapshot, found, err := r.Process(ctx, key)
	if err != nil {
		return err
	}
	if !found || snapshot.Phase != protocol.SandboxProcessRegistered {
		return ErrProcessConflict
	}
	data, _ := json.Marshal(snapshot.Registration)
	return r.transitionProcess(ctx, key, protocol.SandboxProcessRegistered, protocol.SandboxProcessReleased, string(data), "")
}

// AbortPreparedProcess 只撤销尚未登记/放行的意图；迟到登记 CAS 必须失败。
// 它不证明 helper 已退出，调用方仍须撤销对应 job，不能把本状态当作进程回收证据。
func (r *Repository) AbortPreparedProcess(ctx context.Context, key protocol.SandboxProcessKey) error {
	snapshot, found, err := r.Process(ctx, key)
	if err != nil {
		return err
	}
	if !found {
		return ErrProcessConflict
	}
	if snapshot.Phase == protocol.SandboxProcessAborted {
		return nil
	}
	if snapshot.Phase != protocol.SandboxProcessPrepared {
		return ErrProcessConflict
	}
	return r.transitionProcess(ctx, key, protocol.SandboxProcessPrepared, protocol.SandboxProcessAborted, "", "")
}

// ReapProcess 只接受和原登记精确绑定的原生事实，不能由根退出或 job 消失替代。
func (r *Repository) ReapProcess(ctx context.Context, key protocol.SandboxProcessKey, evidence protocol.SandboxProcessEvidence) error {
	snapshot, found, err := r.Process(ctx, key)
	if err != nil {
		return err
	}
	if !found || snapshot.Registration == nil {
		return ErrProcessConflict
	}
	if err := validateProcessEvidence(*snapshot.Registration, evidence); err != nil {
		return err
	}
	if snapshot.Phase == protocol.SandboxProcessReaped {
		if snapshot.Evidence != nil && *snapshot.Evidence == evidence {
			return nil
		}
		return ErrProcessConflict
	}
	if snapshot.Phase != protocol.SandboxProcessRegistered && snapshot.Phase != protocol.SandboxProcessReleased {
		return ErrProcessConflict
	}
	registration, _ := json.Marshal(snapshot.Registration)
	proof, _ := json.Marshal(evidence)
	return r.transitionProcess(ctx, key, snapshot.Phase, protocol.SandboxProcessReaped, string(registration), string(proof))
}

func (r *Repository) transitionProcess(ctx context.Context, k protocol.SandboxProcessKey, from, to protocol.SandboxProcessPhase, registration, evidence string) error {
	if !validProcessKey(k) {
		return ErrInvalidProcess
	}
	b := r.dialect.Bind
	result, err := r.db.ExecContext(ctx, `UPDATE sandbox_process_launches SET phase=`+b(1)+`,registration_json=`+b(2)+`,evidence_json=`+b(3)+`,updated_at=`+b(4)+` WHERE owner_user_id=`+b(5)+` AND session_key=`+b(6)+` AND generation=`+b(7)+` AND launch_id=`+b(8)+` AND phase=`+b(9), to, registration, evidence, time.Now().UTC(), k.OwnerUserID, k.SessionKey, k.Generation, k.LaunchID, from)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrProcessConflict
	}
	return nil
}

func (r *Repository) Process(ctx context.Context, key protocol.SandboxProcessKey) (protocol.SandboxProcessSnapshot, bool, error) {
	if r == nil || r.db == nil {
		return protocol.SandboxProcessSnapshot{}, false, errors.New("sandbox process repository is unavailable")
	}
	if !validProcessKey(key) {
		return protocol.SandboxProcessSnapshot{}, false, ErrInvalidProcess
	}
	b := r.dialect.Bind
	return r.readProcess(ctx, `WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND generation=`+b(3)+` AND launch_id=`+b(4), key.OwnerUserID, key.SessionKey, key.Generation, key.LaunchID)
}
func (r *Repository) LatestProcess(ctx context.Context, owner, session string) (protocol.SandboxProcessSnapshot, bool, error) {
	if r == nil || r.db == nil {
		return protocol.SandboxProcessSnapshot{}, false, errors.New("sandbox process repository is unavailable")
	}
	if owner == "" || session == "" {
		return protocol.SandboxProcessSnapshot{}, false, ErrInvalidProcess
	}
	b := r.dialect.Bind
	return r.readProcess(ctx, `WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` ORDER BY generation DESC LIMIT 1`, owner, session)
}
func (r *Repository) readProcess(ctx context.Context, where string, args ...any) (protocol.SandboxProcessSnapshot, bool, error) {
	var s protocol.SandboxProcessSnapshot
	if r == nil || r.db == nil {
		return s, false, errors.New("sandbox process repository is unavailable")
	}
	var owner, session, launch, intent, registration, evidence string
	var generation int64
	var created, updated any
	err := r.db.QueryRowContext(ctx, `SELECT owner_user_id,session_key,generation,launch_id,phase,intent_json,registration_json,evidence_json,created_at,updated_at FROM sandbox_process_launches `+where, args...).Scan(&owner, &session, &generation, &launch, &s.Phase, &intent, &registration, &evidence, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	if err != nil {
		return s, false, err
	}
	if json.Unmarshal([]byte(intent), &s.Intent) != nil || generation <= 0 || s.Intent.Key != (protocol.SandboxProcessKey{OwnerUserID: owner, SessionKey: session, Generation: uint64(generation), LaunchID: launch}) {
		return protocol.SandboxProcessSnapshot{}, false, ErrInvalidProcess
	}
	if registration != "" {
		if json.Unmarshal([]byte(registration), &s.Registration) != nil || s.Registration == nil {
			return protocol.SandboxProcessSnapshot{}, false, ErrInvalidProcess
		}
	}
	if evidence != "" {
		if json.Unmarshal([]byte(evidence), &s.Evidence) != nil || s.Evidence == nil {
			return protocol.SandboxProcessSnapshot{}, false, ErrInvalidProcess
		}
	}
	for _, item := range []struct {
		value  any
		target *time.Time
	}{{created, &s.CreatedAt}, {updated, &s.UpdatedAt}} {
		parsed, e := storage.NullableTime(item.value)
		if e != nil || parsed == nil {
			return protocol.SandboxProcessSnapshot{}, false, ErrInvalidProcess
		}
		*item.target = *parsed
	}
	if err := validateProcessSnapshot(s); err != nil {
		return protocol.SandboxProcessSnapshot{}, false, err
	}
	return s, true, nil
}
