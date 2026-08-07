// INPUT: Room SQL Session 与每次观察到的 SDK session identity。
// OUTPUT: 不因当前 sdk_session_id 覆盖而丢失的完整 transcript lineage。
// POS: Session 统一视图的 Room transcript 引用持久化边界。
package sessionrepo

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
	for _, sessionID := range protocol.MergeTranscriptSessionIDs(sessionIDs) {
		query := r.dialect.InsertIgnoreInto("session_transcript_refs") + ` (
    room_session_id, sdk_session_id
) VALUES (` + r.dialect.BindList(2) + `)` + r.dialect.InsertIgnoreSuffix()
		if _, err := execer.ExecContext(ctx, query, strings.TrimSpace(roomSessionID), sessionID); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLRepository) attachTranscriptSessionIDs(
	ctx context.Context,
	items []protocol.Session,
) ([]protocol.Session, error) {
	roomSessionIDs := make([]string, 0, len(items))
	indexByID := make(map[string]int, len(items))
	for index := range items {
		if items[index].RoomSessionID == nil {
			continue
		}
		roomSessionID := strings.TrimSpace(*items[index].RoomSessionID)
		if roomSessionID == "" {
			continue
		}
		roomSessionIDs = append(roomSessionIDs, roomSessionID)
		indexByID[roomSessionID] = index
	}
	if len(roomSessionIDs) == 0 {
		return items, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT room_session_id, sdk_session_id
FROM session_transcript_refs
WHERE room_session_id IN (`+r.dialect.BindList(len(roomSessionIDs))+`)
ORDER BY created_at ASC, sdk_session_id ASC`, stringsToAny(roomSessionIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roomSessionID string
		var sdkSessionID string
		if err = rows.Scan(&roomSessionID, &sdkSessionID); err != nil {
			return nil, err
		}
		index, ok := indexByID[roomSessionID]
		if !ok {
			continue
		}
		items[index].TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
			items[index].TranscriptSessionIDs,
			[]string{sdkSessionID},
		)
	}
	return items, rows.Err()
}

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
