// INPUT: inbox 事件与按 event_id 的状态迁移请求。
// OUTPUT: 幂等追加、按 seq 有序的未完成事件，以及只在源状态匹配时生效的迁移。
// POS: inbox 的唯一 SQL 边界；并发正确性全部来自条件更新，不在内存里推断状态。
package inbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 读写 agent_inbox_events。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
	// appendMu 串行化 seq 分配：MAX(seq)+1 在并发追加下会撞 uq_agent_inbox_events_seq；当前只有单实例。
	appendMu sync.Mutex
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// NewRepository 创建 inbox 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}

const eventColumns = `event_id, owner_user_id, inbox_key, seq, kind, dedupe_key, payload, status, COALESCE(round_id, ''), created_at`

// Append 按 (inbox_key, dedupe_key) 幂等追加；重复提交返回原事件与 duplicate=true。
func (r *Repository) Append(ctx context.Context, event Event) (Event, bool, error) {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return Event{}, false, err
	}
	now := time.Now().UTC()
	d := r.dialect
	query := `INSERT INTO agent_inbox_events (
    event_id, owner_user_id, inbox_key, seq, kind, dedupe_key, payload, status, created_at, updated_at
) SELECT ` + d.Bind(1) + `, ` + d.Bind(2) + `, ` + d.Bind(3) + `, COALESCE(MAX(seq), 0) + 1, ` +
		d.Bind(4) + `, ` + d.Bind(5) + `, ` + d.Bind(6) + `, 'pending', ` + d.Bind(7) + `, ` + d.Bind(8) + `
FROM agent_inbox_events WHERE inbox_key = ` + d.Bind(9) + `
ON CONFLICT (inbox_key, dedupe_key) DO NOTHING`
	timestamp := d.TimestampValue(now)
	r.appendMu.Lock()
	defer r.appendMu.Unlock()
	// 读 MAX(seq) 再写必须在同一事务里：SQLite 以 immediate 事务在开始时占住写锁，
	// 否则 WAL 下读升级写会直接 SQLITE_BUSY、绕过 busy_timeout。
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, query,
		event.EventID, event.OwnerUserID, event.InboxKey, event.Kind, event.DedupeKey, string(payload),
		timestamp, timestamp, event.InboxKey)
	if err != nil {
		return Event{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Event{}, false, err
	}
	stored, err := r.loadByDedupe(ctx, tx, event.InboxKey, event.DedupeKey)
	if err != nil {
		return Event{}, false, err
	}
	if stored.OwnerUserID != event.OwnerUserID {
		return Event{}, false, errors.New("inbox dedupe key belongs to another owner")
	}
	return stored, affected == 0, tx.Commit()
}

// ListOpen 返回 inbox 中 pending 与 delivering 的事件，按 seq 升序。
func (r *Repository) ListOpen(ctx context.Context, inboxKey string) ([]Event, error) {
	d := r.dialect
	rows, err := r.db.QueryContext(ctx, `SELECT `+eventColumns+` FROM agent_inbox_events
WHERE inbox_key = `+d.Bind(1)+` AND status IN ('pending', 'delivering') ORDER BY seq`, inboxKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		event, scanErr := scanEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// Transition 把 from 状态的事件推进到 to，返回真正迁移成功的 event_id（保持入参顺序）。
// roundID 写入迁移后的事件；退回 pending 时传空串清除归属。
func (r *Repository) Transition(
	ctx context.Context,
	inboxKey string,
	eventIDs []string,
	from string,
	to string,
	roundID string,
) ([]string, error) {
	d := r.dialect
	query := `UPDATE agent_inbox_events SET status = ` + d.Bind(1) + `, round_id = ` + d.Bind(2) +
		`, updated_at = ` + d.Bind(3) + ` WHERE inbox_key = ` + d.Bind(4) + ` AND event_id = ` + d.Bind(5) +
		` AND status = ` + d.Bind(6)
	now := d.TimestampValue(time.Now().UTC())
	moved := make([]string, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		result, err := r.db.ExecContext(ctx, query, to, nullableString(roundID), now, inboxKey, eventID, from)
		if err != nil {
			return moved, err
		}
		if affected, _ := result.RowsAffected(); affected == 1 {
			moved = append(moved, eventID)
		}
	}
	return moved, nil
}

// ReleaseDelivering 把交给 runtime 却未确认 applied 的事件退回 pending。
// roundID 为空时释放全部（进程重启后，上一进程的 runtime 已不存在）。
func (r *Repository) ReleaseDelivering(ctx context.Context, inboxKey string, roundID string) error {
	d := r.dialect
	query := `UPDATE agent_inbox_events SET status = 'pending', round_id = NULL, updated_at = ` + d.Bind(1) +
		` WHERE status = 'delivering'`
	args := []any{d.TimestampValue(time.Now().UTC())}
	if inboxKey != "" {
		query += ` AND inbox_key = ` + d.Bind(len(args)+1)
		args = append(args, inboxKey)
	}
	if roundID != "" {
		query += ` AND round_id = ` + d.Bind(len(args)+1)
		args = append(args, roundID)
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// PendingInbox 标识一个仍有待处理事件的 inbox。
type PendingInbox struct {
	OwnerUserID string
	InboxKey    string
}

// ListPendingInboxes 返回仍有 pending 事件的 inbox，供启动时唤醒。
func (r *Repository) ListPendingInboxes(ctx context.Context) ([]PendingInbox, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT owner_user_id, inbox_key FROM agent_inbox_events WHERE status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var inboxes []PendingInbox
	for rows.Next() {
		var item PendingInbox
		if err = rows.Scan(&item.OwnerUserID, &item.InboxKey); err != nil {
			return nil, err
		}
		inboxes = append(inboxes, item)
	}
	return inboxes, rows.Err()
}

func (r *Repository) loadByDedupe(ctx context.Context, q queryer, inboxKey string, dedupeKey string) (Event, error) {
	d := r.dialect
	row := q.QueryRowContext(ctx, `SELECT `+eventColumns+` FROM agent_inbox_events
WHERE inbox_key = `+d.Bind(1)+` AND dedupe_key = `+d.Bind(2), inboxKey, dedupeKey)
	return scanEvent(row)
}

func scanEvent(scanner interface{ Scan(...any) error }) (Event, error) {
	var (
		event   Event
		payload string
	)
	if err := scanner.Scan(
		&event.EventID, &event.OwnerUserID, &event.InboxKey, &event.Seq, &event.Kind,
		&event.DedupeKey, &payload, &event.Status, &event.RoundID, &event.CreatedAt,
	); err != nil {
		return Event{}, err
	}
	if err := json.Unmarshal([]byte(payload), &event.Payload); err != nil {
		return Event{}, err
	}
	return event, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
