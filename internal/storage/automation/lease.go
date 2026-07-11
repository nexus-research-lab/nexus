package automation

// 本文件维护 automation scheduler 的数据库租约，避免多个宿主重复触发同一批任务。

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TryAcquireSchedulerLease 获取或续租调度器所有权。
func (r *Repository) TryAcquireSchedulerLease(
	ctx context.Context,
	leaseName string,
	ownerID string,
	now time.Time,
	expiresAt time.Time,
) (bool, error) {
	query := fmt.Sprintf(
		`INSERT INTO automation_scheduler_leases (
    lease_name, owner_id, expires_at, created_at, updated_at
) VALUES (%s, %s, %s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(lease_name) DO UPDATE SET
    owner_id = excluded.owner_id,
    expires_at = excluded.expires_at,
    updated_at = CURRENT_TIMESTAMP
WHERE automation_scheduler_leases.owner_id = excluded.owner_id
   OR automation_scheduler_leases.expires_at <= %s`,
		r.bind(1),
		r.bind(2),
		r.bind(3),
		r.bind(4),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		strings.TrimSpace(leaseName),
		strings.TrimSpace(ownerID),
		expiresAt.UTC(),
		now.UTC(),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ReleaseSchedulerLease 仅释放当前实例持有的调度器租约。
func (r *Repository) ReleaseSchedulerLease(ctx context.Context, leaseName string, ownerID string) error {
	query := fmt.Sprintf(
		"DELETE FROM automation_scheduler_leases WHERE lease_name = %s AND owner_id = %s",
		r.bind(1),
		r.bind(2),
	)
	_, err := r.execWithRetry(ctx, query, strings.TrimSpace(leaseName), strings.TrimSpace(ownerID))
	return err
}
