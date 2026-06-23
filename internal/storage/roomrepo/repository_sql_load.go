package roomrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *SQLRepository) getRoomAggregate(ctx context.Context, querier roomQueryer, ownerUserID string, roomID string) (*protocol.RoomAggregate, error) {
	roomValue, err := r.loadRoom(ctx, querier, ownerUserID, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	members, err := r.listMembers(ctx, querier, roomID)
	if err != nil {
		return nil, err
	}
	return &protocol.RoomAggregate{
		Room:    *roomValue,
		Members: members,
	}, nil
}

func (r *SQLRepository) loadRoom(ctx context.Context, querier roomQueryer, ownerUserID string, roomID string) (*protocol.RoomRecord, error) {
	query := `
SELECT id, owner_user_id, room_type, COALESCE(name, ''), description, COALESCE(avatar, ''), skill_names, COALESCE(host_agent_id, ''), host_auto_reply_enabled, private_messages_enabled, created_at, updated_at
FROM rooms
WHERE id = ` + r.dialect.Bind(1)
	args := []any{roomID}
	if ownerUserID != "" {
		query += ` AND owner_user_id = ` + r.dialect.Bind(2)
		args = append(args, ownerUserID)
	}
	row := querier.QueryRowContext(ctx, query, args...)
	roomValue, err := ScanRoomRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &roomValue, nil
}

func (r *SQLRepository) lookupRoomOwnerUserID(ctx context.Context, querier roomQueryer, roomID string) (string, error) {
	row := querier.QueryRowContext(ctx, `SELECT owner_user_id FROM rooms WHERE id = `+r.dialect.Bind(1)+` LIMIT 1`, roomID)
	var ownerUserID string
	if err := row.Scan(&ownerUserID); errors.Is(err, sql.ErrNoRows) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return ownerUserID, nil
}

func (r *SQLRepository) listMembers(ctx context.Context, querier roomQueryer, roomID string) ([]protocol.MemberRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT id, room_id, member_type, COALESCE(member_user_id, ''), COALESCE(member_agent_id, ''), joined_at
FROM members
WHERE room_id = `+r.dialect.Bind(1)+`
ORDER BY joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.MemberRecord, 0)
	for rows.Next() {
		item, scanErr := ScanMemberRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLRepository) loadRoomsByIDs(
	ctx context.Context,
	querier roomQueryer,
	ownerUserID string,
	roomIDs []string,
) (map[string]protocol.RoomRecord, error) {
	if len(roomIDs) == 0 {
		return map[string]protocol.RoomRecord{}, nil
	}
	query := fmt.Sprintf(`
SELECT id, owner_user_id, room_type, COALESCE(name, ''), description, COALESCE(avatar, ''), skill_names, COALESCE(host_agent_id, ''), host_auto_reply_enabled, private_messages_enabled, created_at, updated_at
FROM rooms
WHERE id IN (%s)`, r.dialect.BindList(len(roomIDs)))
	args := make([]any, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		args = append(args, roomID)
	}
	query += " AND owner_user_id = " + r.dialect.Bind(len(args)+1)
	args = append(args, ownerUserID)
	rows, err := querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]protocol.RoomRecord, len(roomIDs))
	for rows.Next() {
		item, scanErr := ScanRoomRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[item.ID] = item
	}
	return result, rows.Err()
}

func (r *SQLRepository) listMembersByRoomIDs(
	ctx context.Context,
	querier roomQueryer,
	roomIDs []string,
) (map[string][]protocol.MemberRecord, error) {
	if len(roomIDs) == 0 {
		return map[string][]protocol.MemberRecord{}, nil
	}
	query := fmt.Sprintf(`
SELECT id, room_id, member_type, COALESCE(member_user_id, ''), COALESCE(member_agent_id, ''), joined_at
FROM members
WHERE room_id IN (%s)
ORDER BY joined_at ASC`, r.dialect.BindList(len(roomIDs)))
	args := make([]any, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		args = append(args, roomID)
	}
	rows, err := querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]protocol.MemberRecord, len(roomIDs))
	for rows.Next() {
		item, scanErr := ScanMemberRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[item.RoomID] = append(result[item.RoomID], item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, roomID := range roomIDs {
		if _, exists := result[roomID]; !exists {
			result[roomID] = []protocol.MemberRecord{}
		}
	}
	return result, nil
}

func (r *SQLRepository) listMemberAgents(
	ctx context.Context,
	querier roomQueryer,
	roomID string,
) ([]protocol.Agent, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT
    a.id,
    a.name,
    a.owner_user_id,
    a.workspace_path,
    a.status,
    a.is_main,
    COALESCE(a.avatar, ''),
    COALESCE(a.description, ''),
	    COALESCE(`+r.dialect.JSONText("a.vibe_tags")+`, '[]'),
	    a.created_at,
	    COALESCE(rt.provider, ''),
	    COALESCE(rt.model, ''),
	    COALESCE(rt.permission_mode, ''),
    COALESCE(rt.allowed_tools_json, '[]'),
    COALESCE(rt.disallowed_tools_json, '[]'),
    COALESCE(rt.mcp_servers_json, '{}'),
    rt.max_turns,
    rt.max_thinking_tokens,
    COALESCE(rt.setting_sources_json, '[]')
FROM members m
JOIN agents a ON a.id = m.member_agent_id
LEFT JOIN runtimes rt ON rt.agent_id = a.id
WHERE m.room_id = `+r.dialect.Bind(1)+` AND m.member_type = 'agent'
ORDER BY m.joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.Agent, 0)
	for rows.Next() {
		item, scanErr := ScanRoomMemberAgent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLRepository) listConversations(ctx context.Context, querier roomQueryer, roomID string) ([]protocol.ConversationRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT
    c.id,
    c.room_id,
    c.conversation_type,
    COALESCE(c.title, ''),
    (
        SELECT COUNT(1)
        FROM messages m
        WHERE m.conversation_id = c.id
    ),
    c.created_at,
    c.updated_at
FROM conversations c
WHERE room_id = `+r.dialect.Bind(1)+`
ORDER BY created_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.ConversationRecord, 0)
	for rows.Next() {
		item, scanErr := ScanConversationRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLRepository) listSessionsByConversation(ctx context.Context, querier roomQueryer, conversationID string) ([]protocol.SessionRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, COALESCE(sdk_session_id, ''), status, last_activity_at, created_at, updated_at
FROM sessions
WHERE conversation_id = `+r.dialect.Bind(1)+`
ORDER BY last_activity_at DESC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.SessionRecord, 0)
	for rows.Next() {
		item, scanErr := ScanSessionRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLRepository) listSessionsByConversations(ctx context.Context, querier roomQueryer, conversationIDs []string) (map[string][]protocol.SessionRecord, error) {
	result := make(map[string][]protocol.SessionRecord, len(conversationIDs))
	if len(conversationIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(conversationIDs))
	for _, conversationID := range conversationIDs {
		args = append(args, conversationID)
		result[conversationID] = []protocol.SessionRecord{}
	}
	rows, err := querier.QueryContext(ctx, fmt.Sprintf(`
SELECT
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, COALESCE(sdk_session_id, ''), status, last_activity_at, created_at, updated_at
FROM sessions
WHERE conversation_id IN (%s)
ORDER BY conversation_id ASC, last_activity_at DESC`, r.dialect.BindList(len(conversationIDs))), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item, scanErr := ScanSessionRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[item.ConversationID] = append(result[item.ConversationID], item)
	}
	return result, rows.Err()
}

func (r *SQLRepository) pickMainConversation(ctx context.Context, querier roomQueryer, roomID string) (*protocol.ConversationRecord, error) {
	conversations, err := r.listConversations(ctx, querier, roomID)
	if err != nil || len(conversations) == 0 {
		return nil, err
	}
	item := PickMainConversation(conversations)
	return item, nil
}

func (r *SQLRepository) getContextByConversation(ctx context.Context, ownerUserID string, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	contexts, err := r.GetRoomContexts(ctx, ownerUserID, roomID)
	if err != nil {
		return nil, err
	}
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return &contextValue, nil
		}
	}
	if len(contexts) == 0 {
		return nil, nil
	}
	return &contexts[0], nil
}
