// INPUT: 宿主启动恢复的 launch ID 游标与有界页大小。
// OUTPUT: 原始未收口记录的 exact key；不读任务内容、不变更阶段。
// POS: 仅供持独占锁的宿主扫描，JSON 完整性由逐条恢复重读校验。
package sandbox

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// PendingProcessKeys 按唯一 launch ID 分页，避免失败项阻塞后续记录或删除前页导致跳项。
// 此宿主内部跨 owner 扫描不挂载用户 API，也不授予任何执行或恢复权限。
func (r *Repository) PendingProcessKeys(ctx context.Context, after string, limit int) ([]protocol.SandboxProcessKey, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("sandbox process repository is unavailable")
	}
	if limit < 1 || limit > 256 || (after != "" && !validLowerHex(after, 32)) {
		return nil, false, ErrInvalidProcess
	}
	b := r.dialect.Bind
	rows, err := r.db.QueryContext(ctx, `SELECT owner_user_id,session_key,generation,launch_id FROM sandbox_process_launches WHERE phase IN ('prepared','registered','released') AND launch_id>`+b(1)+` ORDER BY launch_id LIMIT `+b(2), after, limit+1)
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
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return keys, false, nil
}
