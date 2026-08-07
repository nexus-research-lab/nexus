// INPUT: 已知的 SDK transcript session ids。
// OUTPUT: 去空、去重并保持首次出现顺序的稳定 lineage。
// POS: 文件 Session 与 Room SQL Session 共享的 transcript 身份语义。
package protocol

import "strings"

// MergeTranscriptSessionIDs 合并并规范化 transcript lineage。
func MergeTranscriptSessionIDs(groups ...[]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, group := range groups {
		for _, sessionID := range group {
			sessionID = strings.ToLower(strings.TrimSpace(sessionID))
			if sessionID == "" {
				continue
			}
			if _, exists := seen[sessionID]; exists {
				continue
			}
			seen[sessionID] = struct{}{}
			result = append(result, sessionID)
		}
	}
	return result
}

// SessionTranscriptIDs 返回 lineage 与当前 SDK session id 的完整并集。
func SessionTranscriptIDs(session Session) []string {
	current := ""
	if session.SessionID != nil {
		current = *session.SessionID
	}
	return MergeTranscriptSessionIDs(session.TranscriptSessionIDs, []string{current})
}

// RoomSessionTranscriptIDs 返回 Room session lineage 与当前 SDK session id 的完整并集。
func RoomSessionTranscriptIDs(session SessionRecord) []string {
	return MergeTranscriptSessionIDs(
		session.TranscriptSessionIDs,
		[]string{session.SDKSessionID},
	)
}
