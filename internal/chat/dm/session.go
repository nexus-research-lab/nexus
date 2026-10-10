// INPUT: Room SQL session index and its Agent workspace runtime projection.
// OUTPUT: One Room-backed Session whose database and workspace fields keep their declared ownership.
// POS: DM domain merge boundary shared by chat execution and the public Session read model.
package dm

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// MergeRoomBackedSession 合并 Room 索引会话与本地 overlay 会话。
func MergeRoomBackedSession(current protocol.Session, roomSession protocol.Session) protocol.Session {
	merged := roomSession
	merged.ConfigurationVersion = current.ConfigurationVersion
	if current.MessageCount > merged.MessageCount {
		merged.MessageCount = current.MessageCount
	}
	if current.LastActivity.After(merged.LastActivity) {
		merged.LastActivity = current.LastActivity
	}
	if textutil.PointerValue(merged.SessionID) == "" && current.SessionID != nil {
		merged.SessionID = current.SessionID
	}
	merged.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
		roomSession.TranscriptSessionIDs,
		protocol.SessionTranscriptIDs(roomSession),
		current.TranscriptSessionIDs,
		protocol.SessionTranscriptIDs(current),
	)
	if current.ContextUsage != nil {
		usage := *current.ContextUsage
		merged.ContextUsage = &usage
	}
	// Room SQL 只拥有 Session 显式覆盖；本地 overlay 继续拥有 runtime 指纹。
	merged.Options = protocol.WithSessionRuntimeSettings(
		current.Options,
		protocol.SessionRuntimeSettingsFromOptions(roomSession.Options),
	)
	for key, value := range roomSession.Options {
		switch key {
		case protocol.OptionSessionProvider,
			protocol.OptionSessionModel,
			protocol.OptionSessionPermissionMode,
			protocol.OptionRuntimeRetainedTranscriptSessionIDs:
			continue
		default:
			merged.Options[key] = value
		}
	}
	// Pending fork 依赖与 Room Session 身份同事务持久化；SQL 缺少该字段
	// 表示 target transcript 已物化，不能被落后的 workspace 投影重新引入。
	for _, key := range []string{
		protocol.OptionRuntimeForkSourceSessionID,
		protocol.OptionRuntimeForkMessageID,
	} {
		if _, exists := roomSession.Options[key]; !exists {
			delete(merged.Options, key)
		}
	}
	delete(merged.Options, protocol.OptionRuntimeRetainedTranscriptSessionIDs)
	return merged
}

// SessionsEqual 判断两个 session 的关键持久字段是否一致。
func SessionsEqual(left protocol.Session, right protocol.Session) bool {
	return left.SessionKey == right.SessionKey &&
		left.AgentID == right.AgentID &&
		textutil.PointerValue(left.SessionID) == textutil.PointerValue(right.SessionID) &&
		textutil.PointerValue(left.RoomSessionID) == textutil.PointerValue(right.RoomSessionID) &&
		textutil.PointerValue(left.RoomID) == textutil.PointerValue(right.RoomID) &&
		textutil.PointerValue(left.ConversationID) == textutil.PointerValue(right.ConversationID) &&
		left.ChannelType == right.ChannelType &&
		left.ChatType == right.ChatType &&
		left.Status == right.Status &&
		left.Title == right.Title &&
		left.LastActivity.Equal(right.LastActivity) &&
		left.MessageCount == right.MessageCount &&
		slices.Equal(left.TranscriptSessionIDs, right.TranscriptSessionIDs) &&
		reflect.DeepEqual(left.ContextUsage, right.ContextUsage) &&
		equivalentJSONValue(left.Options, right.Options)
}

func equivalentJSONValue(left any, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return reflect.DeepEqual(left, right)
	}
	return bytes.Equal(leftJSON, rightJSON)
}
