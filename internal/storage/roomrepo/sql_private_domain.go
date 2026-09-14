// INPUT: Exact owner and Agent identity.
// OUTPUT: All contact-channel Room IDs in which that Agent is a member.
// POS: Private-history lookup only; ordinary room directory filtering stays unchanged.
package roomrepo

import "context"

func (r *SQLRepository) ListAgentContactRoomIDs(ctx context.Context, ownerUserID, agentID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.id FROM rooms r
 WHERE r.owner_user_id = `+r.dialect.Bind(1)+`
 AND r.is_contact_channel = `+r.dialect.TrueValue()+`
 AND EXISTS (SELECT 1 FROM members m WHERE m.room_id = r.id
 AND m.member_type = 'agent' AND m.member_agent_id = `+r.dialect.Bind(2)+`)
 ORDER BY r.updated_at DESC, r.created_at DESC, r.id`, ownerUserID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
