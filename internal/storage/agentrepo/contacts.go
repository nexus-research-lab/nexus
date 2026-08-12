// INPUT: 同 owner 的 Agent 对、各自联系人 id、别名与可选直聊 Room。
// OUTPUT: 双向联系人关系的原子写入、删除、Room 绑定和脱敏目录查询。
// POS: Agent 通讯录的跨方言 SQL 事务边界。
package agentrepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ContactPairRecord 表示一次双向联系人写入。
type ContactPairRecord struct {
	OwnerContactID string
	PeerContactID  string
	OwnerAgentID   string
	ContactAgentID string
	Alias          string
}

// ListAgentContacts 返回一个 Agent 的活跃同 owner 联系人。
func (r *SQLRepository) ListAgentContacts(
	ctx context.Context,
	ownerUserID string,
	ownerAgentID string,
) ([]protocol.AgentContact, error) {
	rows, err := r.db.QueryContext(ctx, r.contactSelect()+`
WHERE owner_agent.owner_user_id = `+r.dialect.Bind(1)+`
  AND contact_agent.owner_user_id = `+r.dialect.Bind(2)+`
  AND c.owner_agent_id = `+r.dialect.Bind(3)+`
  AND contact_agent.status = 'active'
ORDER BY LOWER(COALESCE(NULLIF(c.alias, ''), NULLIF(p.display_name, ''), contact_agent.name)), c.created_at`,
		ownerUserID, ownerUserID, ownerAgentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]protocol.AgentContact, 0)
	for rows.Next() {
		item, scanErr := scanAgentContact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		contacts = append(contacts, item)
	}
	return contacts, rows.Err()
}

// GetAgentContact 返回一个 Agent 的指定联系人。
func (r *SQLRepository) GetAgentContact(
	ctx context.Context,
	ownerUserID string,
	ownerAgentID string,
	contactAgentID string,
) (*protocol.AgentContact, error) {
	row := r.db.QueryRowContext(ctx, r.contactSelect()+`
WHERE owner_agent.owner_user_id = `+r.dialect.Bind(1)+`
  AND contact_agent.owner_user_id = `+r.dialect.Bind(2)+`
  AND c.owner_agent_id = `+r.dialect.Bind(3)+`
  AND c.contact_agent_id = `+r.dialect.Bind(4)+`
  AND contact_agent.status = 'active'`,
		ownerUserID, ownerUserID, ownerAgentID, contactAgentID,
	)
	item, err := scanAgentContact(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpsertAgentContactPair 创建双向联系人；只更新发起方别名。
func (r *SQLRepository) UpsertAgentContactPair(ctx context.Context, record ContactPairRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO contacts (id, owner_agent_id, contact_agent_id, alias)
VALUES (`+r.dialect.BindList(4)+`)
ON CONFLICT (owner_agent_id, contact_agent_id) DO UPDATE SET
    alias = excluded.alias,
    updated_at = `+r.dialect.CurrentTimestamp(),
		record.OwnerContactID, record.OwnerAgentID, record.ContactAgentID, nullIfEmpty(record.Alias),
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
`+r.dialect.InsertIgnoreInto("contacts")+` (id, owner_agent_id, contact_agent_id)
VALUES (`+r.dialect.BindList(3)+`)`+r.dialect.InsertIgnoreSuffix(),
		record.PeerContactID, record.ContactAgentID, record.OwnerAgentID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteAgentContactPair 删除双向联系人，但保留已经产生的 Room 与消息历史。
func (r *SQLRepository) DeleteAgentContactPair(
	ctx context.Context,
	ownerAgentID string,
	contactAgentID string,
) error {
	_, err := r.db.ExecContext(ctx, `
DELETE FROM contacts
WHERE (owner_agent_id = `+r.dialect.Bind(1)+` AND contact_agent_id = `+r.dialect.Bind(2)+`)
   OR (owner_agent_id = `+r.dialect.Bind(3)+` AND contact_agent_id = `+r.dialect.Bind(4)+`)`,
		ownerAgentID, contactAgentID, contactAgentID, ownerAgentID,
	)
	return err
}

// SetAgentContactDirectRoom 把双向联系人绑定到同一条私域 Room。
func (r *SQLRepository) SetAgentContactDirectRoom(
	ctx context.Context,
	ownerAgentID string,
	contactAgentID string,
	roomID string,
) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE contacts
SET direct_room_id = `+r.dialect.Bind(1)+`, updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE (owner_agent_id = `+r.dialect.Bind(2)+` AND contact_agent_id = `+r.dialect.Bind(3)+`)
   OR (owner_agent_id = `+r.dialect.Bind(4)+` AND contact_agent_id = `+r.dialect.Bind(5)+`)`,
		roomID, ownerAgentID, contactAgentID, contactAgentID, ownerAgentID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 2 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLRepository) contactSelect() string {
	return `
SELECT c.id, c.owner_agent_id, c.contact_agent_id,
       COALESCE(c.alias, ''), COALESCE(c.direct_room_id, ''),
       contact_agent.name, COALESCE(p.display_name, ''), COALESCE(contact_agent.avatar, ''),
       c.created_at, c.updated_at
FROM contacts c
JOIN agents owner_agent ON owner_agent.id = c.owner_agent_id
JOIN agents contact_agent ON contact_agent.id = c.contact_agent_id
LEFT JOIN profiles p ON p.agent_id = contact_agent.id`
}

type contactScanner interface {
	Scan(...any) error
}

func scanAgentContact(scanner contactScanner) (protocol.AgentContact, error) {
	var item protocol.AgentContact
	err := scanner.Scan(
		&item.ID, &item.OwnerAgentID, &item.ContactAgentID,
		&item.Alias, &item.DirectRoomID,
		&item.Name, &item.DisplayName, &item.Avatar,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}
