// INPUT: 当前 owner/deployment、Relay Room/message/snapshot/difference 结果。
// OUTPUT: deployment 共享消息、owner 独立游标、幂等且连续的 Nexus 本地投影。
// POS: Relay 权威消息进入 Nexus 单一数据库的本地读模型边界。
package teamrelay

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 保存 Relay 的共享消息和各 owner 的同步游标。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

// NewRepository 创建 Relay 本地投影仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}

// ProjectRoom 建立在线 Room Conversation 的共享投影和 owner 游标。
func (r *Repository) ProjectRoom(
	ctx context.Context,
	ownerUserID string,
	deploymentID string,
	room relaycontract.RoomView,
) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	deploymentID = strings.TrimSpace(deploymentID)
	if ownerUserID == "" || deploymentID == "" {
		return errors.New("Relay 投影缺少 owner 或 deployment")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO team_relay_conversations (
deployment_id, team_id, room_id, conversation_id, stream_id, stream_epoch
) VALUES (` + r.dialect.BindList(6) + `)
ON CONFLICT(deployment_id, conversation_id) DO NOTHING`
	if _, err = tx.ExecContext(ctx, query,
		deploymentID, room.Room.TeamID, room.Room.ID,
		room.Conversation.ID, room.Conversation.SyncStreamID,
		room.Conversation.StreamEpoch,
	); err != nil {
		return err
	}

	// Conversation 是共享写锁；所有 owner 都按这个顺序再更新自己的 cursor。
	if _, err = tx.ExecContext(ctx, `UPDATE team_relay_conversations SET updated_at = updated_at
WHERE deployment_id = `+r.dialect.Bind(1)+` AND conversation_id = `+r.dialect.Bind(2),
		deploymentID, room.Conversation.ID,
	); err != nil {
		return err
	}
	var epoch string
	if err = tx.QueryRowContext(ctx, `SELECT stream_epoch FROM team_relay_conversations
WHERE deployment_id = `+r.dialect.Bind(1)+` AND conversation_id = `+r.dialect.Bind(2),
		deploymentID, room.Conversation.ID,
	).Scan(&epoch); err != nil {
		return err
	}
	if epoch != room.Conversation.StreamEpoch {
		if _, err = tx.ExecContext(ctx, `DELETE FROM team_relay_messages
WHERE deployment_id = `+r.dialect.Bind(1)+` AND conversation_id = `+r.dialect.Bind(2),
			deploymentID, room.Conversation.ID,
		); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE team_relay_conversations SET next_room_seq = 1
WHERE deployment_id = `+r.dialect.Bind(1)+` AND conversation_id = `+r.dialect.Bind(2),
			deploymentID, room.Conversation.ID,
		); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE team_relay_owner_cursors SET relay_seq = 0,
updated_at = CURRENT_TIMESTAMP WHERE deployment_id = `+r.dialect.Bind(1)+`
AND conversation_id = `+r.dialect.Bind(2), deploymentID, room.Conversation.ID); err != nil {
			return err
		}
	}

	query = `UPDATE team_relay_conversations SET team_id = ` + r.dialect.Bind(1) + `,
room_id = ` + r.dialect.Bind(2) + `, stream_id = ` + r.dialect.Bind(3) + `,
stream_epoch = ` + r.dialect.Bind(4) + `, updated_at = CURRENT_TIMESTAMP
WHERE deployment_id = ` + r.dialect.Bind(5) + ` AND conversation_id = ` + r.dialect.Bind(6)
	if _, err = tx.ExecContext(ctx, query,
		room.Room.TeamID, room.Room.ID, room.Conversation.SyncStreamID,
		room.Conversation.StreamEpoch, deploymentID, room.Conversation.ID,
	); err != nil {
		return err
	}
	query = `INSERT INTO team_relay_owner_cursors (
owner_user_id, deployment_id, conversation_id
) VALUES (` + r.dialect.BindList(3) + `)
ON CONFLICT(owner_user_id, deployment_id, conversation_id) DO NOTHING`
	if _, err = tx.ExecContext(ctx, query,
		ownerUserID, deploymentID, room.Conversation.ID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ProjectCommit 幂等投影一条 Relay 消息，并在连续时推进当前 owner 游标。
func (r *Repository) ProjectCommit(
	ctx context.Context,
	ownerUserID string,
	commit relaycontract.MessageCommit,
) error {
	return r.project(ctx, ownerUserID, commit.StreamID, commit.StreamEpoch, func(
		tx *sql.Tx,
		conversation conversationState,
	) (int64, error) {
		if err := r.insertMessage(ctx, tx, conversation, commit.Message); err != nil {
			return conversation.relaySeq, err
		}
		if commit.EventSeq == conversation.relaySeq+1 {
			return commit.EventSeq, nil
		}
		return conversation.relaySeq, nil
	})
}

// ProjectSnapshot 幂等写入一页快照；最后一页才把当前 owner 游标推进到快照水位。
func (r *Repository) ProjectSnapshot(
	ctx context.Context,
	ownerUserID string,
	snapshot relaycontract.Snapshot,
) error {
	messages := append([]relaycontract.Message(nil), snapshot.Messages...)
	sort.Slice(messages, func(i, j int) bool { return messages[i].MessageSeq < messages[j].MessageSeq })
	return r.project(ctx, ownerUserID, snapshot.StreamID, snapshot.StreamEpoch, func(
		tx *sql.Tx,
		conversation conversationState,
	) (int64, error) {
		for _, message := range messages {
			if err := r.insertMessage(ctx, tx, conversation, message); err != nil {
				return conversation.relaySeq, err
			}
		}
		if !snapshot.HasMore && snapshot.SnapshotSeq > conversation.relaySeq {
			return snapshot.SnapshotSeq, nil
		}
		return conversation.relaySeq, nil
	})
}

// ProjectDifference 原子应用连续增量；缺口不会污染当前 owner 游标。
func (r *Repository) ProjectDifference(
	ctx context.Context,
	ownerUserID string,
	difference relaycontract.Difference,
) error {
	return r.project(ctx, ownerUserID, difference.StreamID, difference.StreamEpoch, func(
		tx *sql.Tx,
		conversation conversationState,
	) (int64, error) {
		cursor := conversation.relaySeq
		for _, event := range difference.Events {
			if event.EventSeq <= cursor {
				continue
			}
			if event.EventSeq != cursor+1 {
				return cursor, fmt.Errorf("Relay 增量不连续: got %d, want %d", event.EventSeq, cursor+1)
			}
			if err := r.insertMessage(ctx, tx, conversation, event.Message); err != nil {
				return cursor, err
			}
			cursor = event.EventSeq
		}
		return cursor, nil
	})
}

type conversationState struct {
	deploymentID string
	id           string
	epoch        string
	relaySeq     int64
	streamID     string
}

func (r *Repository) project(
	ctx context.Context,
	ownerUserID string,
	streamID string,
	streamEpoch string,
	apply func(*sql.Tx, conversationState) (int64, error),
) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return errors.New("Relay 投影缺少 owner")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var deploymentID, conversationID string
	err = tx.QueryRowContext(ctx, `SELECT conversation.deployment_id, conversation.conversation_id
FROM team_relay_conversations AS conversation
JOIN team_relay_owner_cursors AS cursor
  ON cursor.deployment_id = conversation.deployment_id
 AND cursor.conversation_id = conversation.conversation_id
WHERE cursor.owner_user_id = `+r.dialect.Bind(1)+` AND conversation.stream_id = `+r.dialect.Bind(2),
		ownerUserID, streamID,
	).Scan(&deploymentID, &conversationID)
	if err != nil {
		return err
	}

	// 共享 Conversation 先加锁，避免不同 owner 重复分配 room_seq。
	result, err := tx.ExecContext(ctx, `UPDATE team_relay_conversations SET updated_at = updated_at
WHERE deployment_id = `+r.dialect.Bind(1)+` AND conversation_id = `+r.dialect.Bind(2),
		deploymentID, conversationID,
	)
	if err != nil {
		return err
	}
	matched, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if matched != 1 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, `UPDATE team_relay_owner_cursors SET updated_at = updated_at
WHERE owner_user_id = `+r.dialect.Bind(1)+` AND deployment_id = `+r.dialect.Bind(2)+`
AND conversation_id = `+r.dialect.Bind(3), ownerUserID, deploymentID, conversationID); err != nil {
		return err
	}

	var conversation conversationState
	err = tx.QueryRowContext(ctx, `SELECT conversation.deployment_id, conversation.conversation_id,
conversation.stream_id, conversation.stream_epoch, cursor.relay_seq
FROM team_relay_conversations AS conversation
JOIN team_relay_owner_cursors AS cursor
  ON cursor.deployment_id = conversation.deployment_id
 AND cursor.conversation_id = conversation.conversation_id
WHERE cursor.owner_user_id = `+r.dialect.Bind(1)+`
AND conversation.deployment_id = `+r.dialect.Bind(2)+`
AND conversation.conversation_id = `+r.dialect.Bind(3), ownerUserID, deploymentID, conversationID).Scan(
		&conversation.deploymentID, &conversation.id, &conversation.streamID,
		&conversation.epoch, &conversation.relaySeq,
	)
	if err != nil {
		return err
	}
	if conversation.epoch != streamEpoch {
		return errors.New("Relay 投影 stream_epoch 不匹配")
	}
	nextRelaySeq, err := apply(tx, conversation)
	if err != nil {
		return err
	}
	if nextRelaySeq != conversation.relaySeq {
		_, err = tx.ExecContext(ctx, `UPDATE team_relay_owner_cursors SET relay_seq = `+
			r.dialect.Bind(1)+`, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = `+
			r.dialect.Bind(2)+` AND deployment_id = `+r.dialect.Bind(3)+`
AND conversation_id = `+r.dialect.Bind(4), nextRelaySeq, ownerUserID,
			conversation.deploymentID, conversation.id,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) insertMessage(
	ctx context.Context,
	tx *sql.Tx,
	conversation conversationState,
	message relaycontract.Message,
) error {
	if message.ConversationID != conversation.id {
		return errors.New("Relay 消息 conversation_id 不匹配")
	}
	content, err := json.Marshal(message.Content)
	if err != nil {
		return err
	}
	var existingSeq int64
	var existingContent []byte
	err = tx.QueryRowContext(ctx, `SELECT message_seq, content_json FROM team_relay_messages
WHERE deployment_id = `+r.dialect.Bind(1)+` AND conversation_id = `+r.dialect.Bind(2)+`
AND message_id = `+r.dialect.Bind(3), conversation.deploymentID, conversation.id, message.ID).Scan(
		&existingSeq, &existingContent,
	)
	if err == nil {
		var existing relaycontract.MessageContent
		if json.Unmarshal(existingContent, &existing) != nil ||
			existingSeq != message.MessageSeq || !reflect.DeepEqual(existing, message.Content) {
			return errors.New("Relay 消息幂等内容冲突")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var roomSeq int64
	err = tx.QueryRowContext(ctx, `UPDATE team_relay_conversations
SET next_room_seq = next_room_seq + 1, updated_at = CURRENT_TIMESTAMP
WHERE deployment_id = `+r.dialect.Bind(1)+` AND conversation_id = `+r.dialect.Bind(2)+`
RETURNING next_room_seq - 1`, conversation.deploymentID, conversation.id).Scan(&roomSeq)
	if err != nil {
		return err
	}
	query := `INSERT INTO team_relay_messages (
deployment_id, conversation_id, message_id, message_seq, room_seq, author_type,
author_user_id, author_username, author_display_name, client_message_id, content_json, created_at
) VALUES (` + r.dialect.BindList(10) + `,` + r.dialect.JSONValue(11) + `,` + r.dialect.Bind(12) + `)`
	_, err = tx.ExecContext(ctx, query,
		conversation.deploymentID, conversation.id, message.ID, message.MessageSeq, roomSeq,
		message.AuthorType, message.AuthorUserID, message.AuthorUsername,
		message.AuthorDisplayName, message.ClientMessageID, string(content),
		r.dialect.TimestampValue(message.CreatedAt),
	)
	return err
}
