// INPUT: Room Session 与每次观察到的 SDK session identity。
// OUTPUT: 不因当前 sdk_session_id 覆盖而丢失的完整 transcript lineage。
// POS: Room 聚合仓储的 transcript 引用持久化边界。
package roomrepo

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type transcriptRefExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (r *SQLRepository) insertTranscriptSessionRefs(
	ctx context.Context,
	execer transcriptRefExecer,
	roomSessionID string,
	sessionIDs ...string,
) error {
	roomSessionID = strings.TrimSpace(roomSessionID)
	if roomSessionID == "" {
		return nil
	}
	for _, sessionID := range protocol.MergeTranscriptSessionIDs(sessionIDs) {
		query := r.dialect.InsertIgnoreInto("session_transcript_refs") + ` (
    room_session_id, sdk_session_id
) VALUES (` + r.dialect.BindList(2) + `)` + r.dialect.InsertIgnoreSuffix()
		if _, err := execer.ExecContext(ctx, query, roomSessionID, sessionID); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLRepository) attachRoomSessionTranscriptIDs(
	ctx context.Context,
	querier roomQueryer,
	items map[string][]protocol.SessionRecord,
) error {
	locations := make(map[string]struct {
		conversationID string
		index          int
	})
	roomSessionIDs := make([]string, 0)
	for conversationID, sessions := range items {
		for index := range sessions {
			roomSessionID := strings.TrimSpace(sessions[index].ID)
			if roomSessionID == "" {
				continue
			}
			roomSessionIDs = append(roomSessionIDs, roomSessionID)
			locations[roomSessionID] = struct {
				conversationID string
				index          int
			}{conversationID: conversationID, index: index}
		}
	}
	if len(roomSessionIDs) == 0 {
		return nil
	}
	args := make([]any, len(roomSessionIDs))
	for index, roomSessionID := range roomSessionIDs {
		args[index] = roomSessionID
	}
	rows, err := querier.QueryContext(ctx, `
SELECT room_session_id, sdk_session_id
FROM session_transcript_refs
WHERE room_session_id IN (`+r.dialect.BindList(len(roomSessionIDs))+`)
ORDER BY created_at ASC, sdk_session_id ASC`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var roomSessionID string
		var sdkSessionID string
		if err = rows.Scan(&roomSessionID, &sdkSessionID); err != nil {
			return err
		}
		location, ok := locations[roomSessionID]
		if !ok {
			continue
		}
		session := &items[location.conversationID][location.index]
		session.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
			session.TranscriptSessionIDs,
			[]string{sdkSessionID},
		)
	}
	return rows.Err()
}
