// INPUT: owner、Room 与可选的 canonical draft conversation ID。
// OUTPUT: Room 内至多一个 is_draft=true 的事务性状态。
// POS: 新建幂等与历史空白页兼容修复共享的 draft 持久化边界。
package roomrepo

import (
	"context"
	"database/sql"
	"errors"
)

// SetRoomDraftConversation 原子设置 Room 的唯一 draft；conversationID 为空时清除 draft。
func (r *SQLRepository) SetRoomDraftConversation(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	conversationID string,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	roomValue, err := r.loadRoom(ctx, tx, ownerUserID, roomID)
	if err != nil {
		return err
	}
	if roomValue == nil {
		return errors.New("room not found")
	}
	if conversationID != "" {
		var found int
		if err = tx.QueryRowContext(
			ctx,
			`SELECT COUNT(1) FROM conversations WHERE id = `+r.dialect.Bind(1)+` AND room_id = `+r.dialect.Bind(2),
			conversationID,
			roomID,
		).Scan(&found); err != nil {
			return err
		}
		if found != 1 {
			return errors.New("draft conversation not found")
		}
	}

	if _, err = tx.ExecContext(
		ctx,
		`UPDATE conversations
SET is_draft = `+r.dialect.FalseValue()+`
WHERE room_id = `+r.dialect.Bind(1)+` AND is_draft = `+r.dialect.TrueValue(),
		roomID,
	); err != nil {
		return err
	}
	if conversationID != "" {
		result, updateErr := tx.ExecContext(
			ctx,
			`UPDATE conversations
SET is_draft = `+r.dialect.TrueValue()+`, updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(1)+` AND room_id = `+r.dialect.Bind(2),
			conversationID,
			roomID,
		)
		if updateErr != nil {
			return updateErr
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if affected != 1 {
			return sql.ErrNoRows
		}
	}
	return tx.Commit()
}
