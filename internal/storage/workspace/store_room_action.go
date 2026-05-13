package workspace

import (
	"errors"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomActionContextLimit = 20

// RoomActionStore 负责 Room action 的 append-only 读写。
type RoomActionStore struct {
	paths *Store
	files *SessionFileStore
}

// NewRoomActionStore 创建 Room action 存储。
func NewRoomActionStore(root string) *RoomActionStore {
	return &RoomActionStore{
		paths: New(root),
		files: NewSessionFileStore(root),
	}
}

// AppendAction 追加一条 Room action。
func (s *RoomActionStore) AppendAction(action protocol.RoomActionRecord) error {
	return s.files.appendJSONL(s.paths.RoomConversationActionsPath(action.ConversationID), roomActionToRow(action))
}

// ReadActions 读取指定对话的全部 Room action。
func (s *RoomActionStore) ReadActions(conversationID string) ([]protocol.RoomActionRecord, error) {
	rows, err := s.files.readJSONL(s.paths.RoomConversationActionsPath(conversationID))
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.RoomActionRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	actions := make([]protocol.RoomActionRecord, 0, len(rows))
	for _, row := range rows {
		action := roomActionFromRow(row)
		if strings.TrimSpace(action.ActionID) == "" {
			continue
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// ReadContextActions 读取对目标 agent 可见的近期 action。
func (s *RoomActionStore) ReadContextActions(conversationID string, agentID string) ([]protocol.RoomActionRecord, error) {
	actions, err := s.ReadActions(conversationID)
	if err != nil {
		return nil, err
	}
	targetAgentID := strings.TrimSpace(agentID)
	visible := make([]protocol.RoomActionRecord, 0, len(actions))
	for _, action := range actions {
		if roomActionVisibleToAgent(action, targetAgentID) {
			visible = append(visible, action)
		}
	}
	if len(visible) > roomActionContextLimit {
		visible = visible[len(visible)-roomActionContextLimit:]
	}
	return visible, nil
}

func roomActionVisibleToAgent(action protocol.RoomActionRecord, agentID string) bool {
	if agentID == "" {
		return false
	}
	if action.Visibility == protocol.RoomActionVisibilityPublic || action.ReplyTarget == protocol.RoomReplyTargetPublicFeed {
		return true
	}
	switch action.ReplyTarget {
	case protocol.RoomReplyTargetSenderPrivate:
		return action.SourceAgentID == agentID
	case protocol.RoomReplyTargetTargetPrivate:
		return action.TargetAgentID == agentID
	case protocol.RoomReplyTargetAudience:
		return containsRoomActionAgent(action.AudienceAgentIDs, agentID)
	default:
		return action.TargetAgentID == agentID
	}
}

func roomActionToRow(action protocol.RoomActionRecord) map[string]any {
	row := map[string]any{
		"action_id":       action.ActionID,
		"room_id":         action.RoomID,
		"conversation_id": action.ConversationID,
		"action_type":     string(action.ActionType),
		"source_agent_id": action.SourceAgentID,
		"visibility":      action.Visibility,
		"reply_target":    string(action.ReplyTarget),
		"timestamp":       action.Timestamp,
	}
	if strings.TrimSpace(action.TargetAgentID) != "" {
		row["target_agent_id"] = action.TargetAgentID
	}
	if len(action.AudienceAgentIDs) > 0 {
		row["audience_agent_ids"] = append([]string(nil), action.AudienceAgentIDs...)
	}
	if strings.TrimSpace(action.Content) != "" {
		row["content"] = action.Content
	}
	return row
}

func roomActionFromRow(row map[string]any) protocol.RoomActionRecord {
	return protocol.RoomActionRecord{
		ActionID:         stringFromAny(row["action_id"]),
		RoomID:           stringFromAny(row["room_id"]),
		ConversationID:   stringFromAny(row["conversation_id"]),
		ActionType:       protocol.RoomActionType(stringFromAny(row["action_type"])),
		SourceAgentID:    stringFromAny(row["source_agent_id"]),
		TargetAgentID:    stringFromAny(row["target_agent_id"]),
		AudienceAgentIDs: stringSliceFromAny(row["audience_agent_ids"]),
		Content:          stringFromAny(row["content"]),
		Visibility:       stringFromAny(row["visibility"]),
		ReplyTarget:      protocol.RoomReplyTarget(stringFromAny(row["reply_target"])),
		Timestamp:        int64FromAny(row["timestamp"]),
	}
}

func containsRoomActionAgent(items []string, value string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}
