package workspace

import (
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomActionContextLimit = 20

// RoomActionCursor 记录某个 Room agent 已消费到的 action 位置。
type RoomActionCursor struct {
	RoomID              string
	ConversationID      string
	AgentID             string
	RoundID             string
	LastActionID        string
	LastActionTimestamp int64
	Timestamp           int64
}

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
	return s.ReadContextActionsAfterCursor(conversationID, agentID, RoomActionCursor{})
}

// ReadVisibleActions 读取目标 agent 可见的全部 action，不裁剪上下文窗口。
func (s *RoomActionStore) ReadVisibleActions(conversationID string, agentID string) ([]protocol.RoomActionRecord, error) {
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
	return visible, nil
}

// ReadContextActionsAfterCursor 读取目标 agent cursor 之后可见的近期 action。
func (s *RoomActionStore) ReadContextActionsAfterCursor(
	conversationID string,
	agentID string,
	cursor RoomActionCursor,
) ([]protocol.RoomActionRecord, error) {
	visible, err := s.ReadVisibleActions(conversationID, agentID)
	if err != nil {
		return nil, err
	}
	visible = roomActionsAfterCursor(visible, cursor)
	if len(visible) > roomActionContextLimit {
		visible = visible[len(visible)-roomActionContextLimit:]
	}
	return visible, nil
}

// AppendActionCursor 追加 Room action 消费位置控制行。
func (s *RoomActionStore) AppendActionCursor(cursor RoomActionCursor) error {
	return s.files.appendJSONL(s.paths.RoomConversationActionCursorsPath(cursor.ConversationID), roomActionCursorToRow(cursor))
}

// ReadActionCursor 读取目标 agent 最新 Room action 消费位置。
func (s *RoomActionStore) ReadActionCursor(conversationID string, agentID string) (RoomActionCursor, bool, error) {
	if strings.TrimSpace(agentID) == "" {
		return RoomActionCursor{}, false, nil
	}
	cursors, err := s.ReadActionCursors(conversationID, agentID)
	if err != nil {
		return RoomActionCursor{}, false, err
	}
	if len(cursors) == 0 {
		return RoomActionCursor{}, false, nil
	}
	return cursors[0], true, nil
}

// ReadActionCursors 读取每个 agent 最新的 Room action 消费位置。
func (s *RoomActionStore) ReadActionCursors(conversationID string, agentID string) ([]RoomActionCursor, error) {
	rows, err := s.files.readJSONL(s.paths.RoomConversationActionCursorsPath(conversationID))
	if errors.Is(err, os.ErrNotExist) {
		return []RoomActionCursor{}, nil
	}
	if err != nil {
		return nil, err
	}
	targetAgentID := strings.TrimSpace(agentID)
	latestByAgentID := map[string]RoomActionCursor{}
	for _, row := range rows {
		cursor := roomActionCursorFromRow(row)
		cursorAgentID := strings.TrimSpace(cursor.AgentID)
		if cursorAgentID == "" ||
			strings.TrimSpace(cursor.ConversationID) != strings.TrimSpace(conversationID) ||
			strings.TrimSpace(cursor.LastActionID) == "" ||
			(targetAgentID != "" && cursorAgentID != targetAgentID) {
			continue
		}
		latestByAgentID[cursorAgentID] = cursor
	}
	agentIDs := make([]string, 0, len(latestByAgentID))
	for cursorAgentID := range latestByAgentID {
		agentIDs = append(agentIDs, cursorAgentID)
	}
	sort.Strings(agentIDs)
	cursors := make([]RoomActionCursor, 0, len(agentIDs))
	for _, cursorAgentID := range agentIDs {
		cursors = append(cursors, latestByAgentID[cursorAgentID])
	}
	return cursors, nil
}

func roomActionVisibleToAgent(action protocol.RoomActionRecord, agentID string) bool {
	if agentID == "" {
		return false
	}
	switch action.ActionType {
	case protocol.RoomActionTypePrivateMessage:
		if strings.TrimSpace(action.TargetAgentID) != "" {
			return action.TargetAgentID == agentID
		}
		return containsRoomActionAgent(action.AudienceAgentIDs, agentID)
	case protocol.RoomActionTypeRequestReply:
		return action.TargetAgentID == agentID
	case protocol.RoomActionTypePrivateNote:
		return action.SourceAgentID == agentID
	}
	if action.ReplyTarget == protocol.RoomReplyTargetNone {
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

func roomActionsAfterCursor(actions []protocol.RoomActionRecord, cursor RoomActionCursor) []protocol.RoomActionRecord {
	if len(actions) == 0 {
		return nil
	}
	cursorActionID := strings.TrimSpace(cursor.LastActionID)
	if cursorActionID != "" {
		for index, action := range actions {
			if strings.TrimSpace(action.ActionID) == cursorActionID {
				return actions[index+1:]
			}
		}
	}
	if cursor.LastActionTimestamp <= 0 {
		return actions
	}
	result := make([]protocol.RoomActionRecord, 0, len(actions))
	for _, action := range actions {
		if action.Timestamp > cursor.LastActionTimestamp {
			result = append(result, action)
		}
	}
	return result
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
	if strings.TrimSpace(action.RequestID) != "" {
		row["request_id"] = action.RequestID
	}
	if action.WakePolicy != "" {
		row["wake_policy"] = string(action.WakePolicy)
	}
	if action.DelaySeconds > 0 {
		row["delay_seconds"] = action.DelaySeconds
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

func roomActionCursorToRow(cursor RoomActionCursor) map[string]any {
	return map[string]any{
		"room_id":               strings.TrimSpace(cursor.RoomID),
		"conversation_id":       strings.TrimSpace(cursor.ConversationID),
		"agent_id":              strings.TrimSpace(cursor.AgentID),
		"round_id":              strings.TrimSpace(cursor.RoundID),
		"last_action_id":        strings.TrimSpace(cursor.LastActionID),
		"last_action_timestamp": cursor.LastActionTimestamp,
		"timestamp":             cursor.Timestamp,
	}
}

func roomActionCursorFromRow(row map[string]any) RoomActionCursor {
	return RoomActionCursor{
		RoomID:              stringFromAny(row["room_id"]),
		ConversationID:      stringFromAny(row["conversation_id"]),
		AgentID:             stringFromAny(row["agent_id"]),
		RoundID:             stringFromAny(row["round_id"]),
		LastActionID:        stringFromAny(row["last_action_id"]),
		LastActionTimestamp: int64FromAny(row["last_action_timestamp"]),
		Timestamp:           int64FromAny(row["timestamp"]),
	}
}

func roomActionFromRow(row map[string]any) protocol.RoomActionRecord {
	return protocol.RoomActionRecord{
		ActionID:         stringFromAny(row["action_id"]),
		RoomID:           stringFromAny(row["room_id"]),
		ConversationID:   stringFromAny(row["conversation_id"]),
		ActionType:       protocol.RoomActionType(stringFromAny(row["action_type"])),
		RequestID:        stringFromAny(row["request_id"]),
		SourceAgentID:    stringFromAny(row["source_agent_id"]),
		TargetAgentID:    stringFromAny(row["target_agent_id"]),
		AudienceAgentIDs: stringSliceFromAny(row["audience_agent_ids"]),
		Content:          stringFromAny(row["content"]),
		Visibility:       stringFromAny(row["visibility"]),
		ReplyTarget:      protocol.RoomReplyTarget(stringFromAny(row["reply_target"])),
		WakePolicy:       protocol.RoomWakePolicy(stringFromAny(row["wake_policy"])),
		DelaySeconds:     int(int64FromAny(row["delay_seconds"])),
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
