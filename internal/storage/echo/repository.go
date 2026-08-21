// INPUT: Echo 尝试领域对象与 owner/session 生命周期命令。
// OUTPUT: SQLite/Postgres 一致的 attempt 持久化与原子状态迁移。
// POS: Echo durable attempt 的唯一数据库访问边界。
package echo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 保存 Echo durable attempt。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

// NewRepository 创建 Echo 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}

// InsertAttempt 保存由终态 round 锚定的唯一待执行尝试。
func (r *Repository) InsertAttempt(ctx context.Context, item echodomain.Attempt) (bool, error) {
	query := `INSERT INTO echo_attempts (
attempt_id, owner_user_id, agent_id, session_key, trigger_kind, anchor_round_id,
anchor_message_id, anchor_finished_at, due_at, expires_at, status, created_at, updated_at
) VALUES (` + r.dialect.BindList(11) + `,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(owner_user_id, session_key, trigger_kind, anchor_round_id) DO NOTHING`
	result, err := r.db.ExecContext(
		ctx,
		query,
		item.AttemptID,
		item.OwnerUserID,
		item.AgentID,
		item.SessionKey,
		item.TriggerKind,
		item.AnchorRoundID,
		nullString(item.AnchorMessageID),
		r.dialect.TimestampValue(item.AnchorFinishedAt),
		r.dialect.TimestampValue(item.DueAt),
		r.dialect.TimestampValue(item.ExpiresAt),
		echodomain.StatusScheduled,
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// NextDueAt 返回最早的待判断时间。
func (r *Repository) NextDueAt(ctx context.Context) (*time.Time, error) {
	query := `SELECT MIN(due_at) FROM echo_attempts WHERE status = 'scheduled'`
	var value sql.NullTime
	if err := r.db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return nil, err
	}
	return nullTime(value), nil
}

// ClaimDue 原子领取一条到期尝试。
func (r *Repository) ClaimDue(ctx context.Context, now time.Time) (*echodomain.Attempt, error) {
	query := `UPDATE echo_attempts SET
status = 'evaluating', started_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE attempt_id = (
    SELECT attempt_id FROM echo_attempts
    WHERE status = 'scheduled' AND due_at <= ` + r.dialect.Bind(1) + `
    ORDER BY due_at ASC, created_at ASC LIMIT 1
) AND status = 'scheduled'
RETURNING ` + attemptColumns
	return scanOptionalAttempt(r.db.QueryRowContext(ctx, query, r.dialect.TimestampValue(now)))
}

// Reschedule 把尚未开始的尝试移动到下一个有效窗口。
func (r *Repository) Reschedule(ctx context.Context, attemptID string, dueAt time.Time, reason string) error {
	query := `UPDATE echo_attempts SET status = 'scheduled', due_at = ` + r.dialect.Bind(1) + `,
decision_reason = ` + r.dialect.Bind(2) + `, started_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE attempt_id = ` + r.dialect.Bind(3) + ` AND status = 'evaluating'`
	_, err := r.db.ExecContext(ctx, query, r.dialect.TimestampValue(dueAt), nullString(reason), strings.TrimSpace(attemptID))
	return err
}

// MarkRunning 记录已通过 Gate、即将启动的 runtime round。
func (r *Repository) MarkRunning(ctx context.Context, attemptID string, roundID string, reason string, focus string) (bool, error) {
	query := `UPDATE echo_attempts SET status = 'running', runtime_round_id = ` + r.dialect.Bind(1) + `,
decision_reason = ` + r.dialect.Bind(2) + `, focus = ` + r.dialect.Bind(3) + `, updated_at = CURRENT_TIMESTAMP
WHERE attempt_id = ` + r.dialect.Bind(4) + ` AND status = 'evaluating'`
	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(roundID), nullString(reason), nullString(focus), strings.TrimSpace(attemptID))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// AdmitCommit 是用户可见消息写入前的最终 durable gate。
func (r *Repository) AdmitCommit(ctx context.Context, attemptID string, messageID string) (bool, error) {
	query := `UPDATE echo_attempts SET status = 'committing', delivered_message_id = ` + r.dialect.Bind(1) + `,
updated_at = CURRENT_TIMESTAMP WHERE attempt_id = ` + r.dialect.Bind(2) + ` AND status = 'running'`
	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(messageID), strings.TrimSpace(attemptID))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// FinishCommit 收口消息持久化结果。
func (r *Repository) FinishCommit(ctx context.Context, attemptID string, commitErr error) error {
	status := echodomain.StatusDelivered
	errorCode := any(nil)
	if commitErr != nil {
		status = echodomain.StatusFailed
		errorCode = "delivery_failed"
	}
	query := `UPDATE echo_attempts SET status = ` + r.dialect.Bind(1) + `,
error_code = ` + r.dialect.Bind(2) + `, finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE attempt_id = ` + r.dialect.Bind(3) + ` AND status = 'committing'`
	_, err := r.db.ExecContext(ctx, query, status, errorCode, strings.TrimSpace(attemptID))
	return err
}

// FinishWithoutDelivery 收口抑制、取消或失败状态。
func (r *Repository) FinishWithoutDelivery(ctx context.Context, attemptID string, status string, reason string, errorCode string) error {
	query := `UPDATE echo_attempts SET status = ` + r.dialect.Bind(1) + `,
decision_reason = ` + r.dialect.Bind(2) + `, error_code = ` + r.dialect.Bind(3) + `,
finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE attempt_id = ` + r.dialect.Bind(4) + ` AND status IN ('evaluating', 'running')`
	_, err := r.db.ExecContext(ctx, query, status, nullString(reason), nullString(errorCode), strings.TrimSpace(attemptID))
	return err
}

// CancelSession 取消一次用户活动之前尚未提交的 Echo，并返回需精确中断的 round。
func (r *Repository) CancelSession(ctx context.Context, ownerUserID string, sessionKey string) ([]string, error) {
	query := `UPDATE echo_attempts SET status = 'cancelled', decision_reason = 'user_activity',
finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = ` + r.dialect.Bind(1) + ` AND session_key = ` + r.dialect.Bind(2) + `
AND status IN ('scheduled', 'evaluating', 'running')
RETURNING runtime_round_id`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(sessionKey))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roundIDs []string
	for rows.Next() {
		var value sql.NullString
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		if value.Valid && strings.TrimSpace(value.String) != "" {
			roundIDs = append(roundIDs, strings.TrimSpace(value.String))
		}
	}
	return roundIDs, rows.Err()
}

// CancelOwner 取消全局停用后当前用户尚未提交的全部尝试。
func (r *Repository) CancelOwner(ctx context.Context, ownerUserID string) ([]string, error) {
	query := `UPDATE echo_attempts SET status = 'cancelled', decision_reason = 'policy_disabled',
finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = ` + r.dialect.Bind(1) + `
AND status IN ('scheduled', 'evaluating', 'running') RETURNING runtime_round_id`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuntimeRoundIDs(rows)
}

// HasOtherActiveForAgent 判断同 Agent 是否已有在途尝试。
func (r *Repository) HasOtherActiveForAgent(ctx context.Context, ownerUserID string, agentID string, attemptID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM echo_attempts WHERE owner_user_id = ` + r.dialect.Bind(1) + `
AND agent_id = ` + r.dialect.Bind(2) + ` AND attempt_id <> ` + r.dialect.Bind(3) + `
AND status IN ('evaluating', 'running', 'committing'))`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(agentID), strings.TrimSpace(attemptID)).Scan(&exists)
	return exists, err
}

// RecoverInFlight 把进程重启前未收口的尝试标记为失败。
func (r *Repository) RecoverInFlight(ctx context.Context) error {
	query := `UPDATE echo_attempts SET status = 'failed', error_code = 'process_restarted',
finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE status IN ('evaluating', 'running', 'committing')`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// GetAttempt 读取单次尝试。
func (r *Repository) GetAttempt(ctx context.Context, attemptID string) (*echodomain.Attempt, error) {
	query := `SELECT ` + attemptColumns + ` FROM echo_attempts WHERE attempt_id = ` + r.dialect.Bind(1)
	return scanOptionalAttempt(r.db.QueryRowContext(ctx, query, strings.TrimSpace(attemptID)))
}

// GetAttemptByRuntimeRoundID 按运行轮次读取尝试，用于策略停用后的精确中断。
func (r *Repository) GetAttemptByRuntimeRoundID(ctx context.Context, roundID string) (*echodomain.Attempt, error) {
	query := `SELECT ` + attemptColumns + ` FROM echo_attempts WHERE runtime_round_id = ` + r.dialect.Bind(1)
	return scanOptionalAttempt(r.db.QueryRowContext(ctx, query, strings.TrimSpace(roundID)))
}

// CountDeliveredSince 返回当前用户本地日窗口内已投递次数。
func (r *Repository) CountDeliveredSince(ctx context.Context, ownerUserID string, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM echo_attempts WHERE owner_user_id = ` + r.dialect.Bind(1) + `
AND status = 'delivered' AND finished_at >= ` + r.dialect.Bind(2)
	var count int
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), r.dialect.TimestampValue(since)).Scan(&count)
	return count, err
}

// LastDeliveredAtForSession 返回同一私聊最近一次主动消息时间。
func (r *Repository) LastDeliveredAtForSession(ctx context.Context, ownerUserID string, sessionKey string) (*time.Time, error) {
	query := `SELECT MAX(finished_at) FROM echo_attempts WHERE owner_user_id = ` + r.dialect.Bind(1) + `
AND session_key = ` + r.dialect.Bind(2) + ` AND status = 'delivered'`
	var value sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(sessionKey)).Scan(&value); err != nil {
		return nil, err
	}
	return nullTime(value), nil
}

const attemptColumns = `attempt_id, owner_user_id, agent_id, session_key, trigger_kind,
anchor_round_id, anchor_message_id, anchor_finished_at, due_at, expires_at, status, runtime_round_id,
decision_reason, focus, error_code, delivered_message_id, started_at, finished_at, created_at`

type scanner interface {
	Scan(...any) error
}

func scanOptionalAttempt(row *sql.Row) (*echodomain.Attempt, error) {
	item, err := scanAttempt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAttempt(source scanner) (echodomain.Attempt, error) {
	var item echodomain.Attempt
	var anchorMessageID, runtimeRoundID, reason, focus, errorCode, deliveredMessageID sql.NullString
	var startedAt, finishedAt sql.NullTime
	err := source.Scan(
		&item.AttemptID,
		&item.OwnerUserID,
		&item.AgentID,
		&item.SessionKey,
		&item.TriggerKind,
		&item.AnchorRoundID,
		&anchorMessageID,
		&item.AnchorFinishedAt,
		&item.DueAt,
		&item.ExpiresAt,
		&item.Status,
		&runtimeRoundID,
		&reason,
		&focus,
		&errorCode,
		&deliveredMessageID,
		&startedAt,
		&finishedAt,
		&item.CreatedAt,
	)
	item.AnchorMessageID = nullableString(anchorMessageID)
	item.RuntimeRoundID = nullableString(runtimeRoundID)
	item.DecisionReason = nullableString(reason)
	item.Focus = nullableString(focus)
	item.ErrorCode = nullableString(errorCode)
	item.DeliveredMessageID = nullableString(deliveredMessageID)
	item.StartedAt = nullTime(startedAt)
	item.FinishedAt = nullTime(finishedAt)
	return item, err
}

func nullString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func nullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func scanRuntimeRoundIDs(rows *sql.Rows) ([]string, error) {
	var roundIDs []string
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value.Valid && strings.TrimSpace(value.String) != "" {
			roundIDs = append(roundIDs, strings.TrimSpace(value.String))
		}
	}
	return roundIDs, rows.Err()
}
