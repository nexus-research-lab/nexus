// INPUT: scheduler owner、当前时间与期望租约期限。
// OUTPUT: 原子 leader acquire/renew/release 及当前 holder expiry deadline。
// POS: Automation 多实例调度互斥边界；service 负责 timer 和 takeover 决策。
package automation

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

// SchedulerLeaseExpiresAt 返回当前 holder 的持久期限；不存在时返回 nil。
func (r *Repository) SchedulerLeaseExpiresAt(
	ctx context.Context,
	leaseName string,
) (*time.Time, error) {
	var expiresAt any
	err := r.db.QueryRowContext(
		ctx,
		"SELECT expires_at FROM automation_scheduler_leases WHERE lease_name = "+r.bind(1),
		strings.TrimSpace(leaseName),
	).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return storage.NullableTime(expiresAt)
}

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
