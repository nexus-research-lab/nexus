// INPUT: 已落盘的完整 assistant 回复、owner 与来源 conversation/session。
// OUTPUT: 每个 DM/Room 唯一的当前短摘要及删除/重写失效栅栏。
// POS: 首屏摘要独立 SQL 投影；不读取或依赖历史索引。
package roomrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RecordReplyPreview 只接收持久完整回复；私有成员历史只能更新 DM，不能覆盖群聊公区。
func (r *SQLRepository) RecordReplyPreview(ctx context.Context, owner, conversation, sessionKey string, public bool, message protocol.Message) error {
	if r == nil || r.db == nil || message["is_complete"] != true {
		return nil
	}
	preview := messageutil.LatestReplyPreview([]protocol.Message{message})
	if preview == "" {
		return nil
	}
	messageID, _ := message["message_id"].(string)
	if strings.TrimSpace(messageID) == "" {
		return nil
	}
	var timestamp int64
	switch value := message["timestamp"].(type) {
	case int64:
		timestamp = value
	case int:
		timestamp = int64(value)
	case float64:
		timestamp = int64(value)
	}
	if timestamp <= 0 {
		return nil
	}
	query := fmt.Sprintf(`INSERT INTO room_reply_previews
	 (owner_user_id, room_id, conversation_id, session_key, message_id, preview, reply_timestamp)
	 SELECT r.owner_user_id, r.id, c.id, %s, %s, %s, %s
	 FROM rooms r JOIN conversations c ON c.room_id = r.id
	 WHERE r.owner_user_id = %s AND c.id = %s`,
		r.dialect.Bind(1), r.dialect.Bind(2), r.dialect.Bind(3), r.dialect.Bind(4), r.dialect.Bind(5), r.dialect.Bind(6))
	if !public {
		query += ` AND r.room_type = 'dm'`
	}
	query += ` ON CONFLICT(owner_user_id, room_id) DO UPDATE SET
	 conversation_id = excluded.conversation_id, session_key = excluded.session_key,
	 message_id = excluded.message_id, preview = excluded.preview, reply_timestamp = excluded.reply_timestamp
	 WHERE excluded.reply_timestamp > room_reply_previews.invalidated_before
 AND (excluded.reply_timestamp > room_reply_previews.reply_timestamp
	 OR (excluded.reply_timestamp = room_reply_previews.reply_timestamp
 AND room_reply_previews.preview <> '' AND excluded.message_id = room_reply_previews.message_id
 AND excluded.preview <> room_reply_previews.preview))`
	_, err := r.db.ExecContext(ctx, query, sessionKey, messageID, preview, timestamp, owner, conversation)
	return err
}

// ListRoomReplyPreviews 一次读取本 owner 的短摘要，不打开任何会话历史文件。
func (r *SQLRepository) ListRoomReplyPreviews(ctx context.Context, owner string) (map[string]string, error) {
	items := make(map[string]string)
	if r == nil || r.db == nil {
		return items, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT room_id, preview FROM room_reply_previews WHERE owner_user_id = `+r.dialect.Bind(1)+` AND preview <> ''`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roomID, preview string
		if err = rows.Scan(&roomID, &preview); err != nil {
			return nil, err
		}
		items[roomID] = preview
	}
	return items, rows.Err()
}

// InvalidateReplyPreview 保留 Room 级时间栅栏，连非当前来源的延迟写回也不能复活旧内容。
func (r *SQLRepository) InvalidateReplyPreview(ctx context.Context, owner, sessionKey string) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.InvalidateReplyPreviewsTx(ctx, nil, owner, []string{sessionKey})
}

// InvalidateReplyPreviewsTx 与 Session 的其他次级数据在同一个删除事务中失效。
func (r *SQLRepository) InvalidateReplyPreviewsTx(ctx context.Context, tx *sql.Tx, owner string, sessionKeys []string) error {
	for _, key := range sessionKeys {
		parsed := protocol.ParseSessionKey(key)
		conversation := parsed.Ref
		if conversation == "" {
			continue
		}
		query := fmt.Sprintf(`INSERT INTO room_reply_previews
          (owner_user_id, room_id, conversation_id, session_key, message_id, preview, reply_timestamp, invalidated_before)
          SELECT r.owner_user_id, r.id, c.id, %s, '', '', 0, %s
          FROM rooms r JOIN conversations c ON c.room_id = r.id
          WHERE r.owner_user_id = %s AND c.id = %s
          ON CONFLICT(owner_user_id, room_id) DO UPDATE SET
          preview = CASE WHEN room_reply_previews.session_key = excluded.session_key THEN '' ELSE room_reply_previews.preview END,
          invalidated_before = CASE WHEN room_reply_previews.invalidated_before > excluded.invalidated_before
           THEN room_reply_previews.invalidated_before ELSE excluded.invalidated_before END`,
			r.dialect.Bind(1), r.dialect.Bind(2), r.dialect.Bind(3), r.dialect.Bind(4))
		var err error
		args := []any{key, time.Now().UnixMilli(), owner, conversation}
		if tx != nil {
			_, err = tx.ExecContext(ctx, query, args...)
		} else {
			_, err = r.db.ExecContext(ctx, query, args...)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
