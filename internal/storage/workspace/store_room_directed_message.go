package workspace

import (
	"errors"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomDirectedMessageContextLimit = 20

// RoomDirectedMessageCursor 记录某个 Room agent 已消费到的 directed message 位置。
type RoomDirectedMessageCursor struct {
	RoomID               string
	ConversationID       string
	AgentID              string
	RoundID              string
	LastMessageID        string
	LastMessageTimestamp int64
	Timestamp            int64
}

// RoomDirectedMessageStore 负责 Room directed message 的 append-only 读写。
type RoomDirectedMessageStore struct {
	paths *Store
	files *SessionFileStore
}

// NewRoomDirectedMessageStore 创建 Room directed message 存储。
func NewRoomDirectedMessageStore(root string) *RoomDirectedMessageStore {
	return &RoomDirectedMessageStore{
		paths: New(root),
		files: NewSessionFileStore(root),
	}
}

// AppendMessage 追加一条 Room directed message。
func (s *RoomDirectedMessageStore) AppendMessage(message protocol.RoomDirectedMessageRecord) error {
	return s.files.appendJSONL(s.paths.RoomConversationMessagesPath(message.ConversationID), roomDirectedMessageToRow(message))
}

// ReadMessages 读取指定对话的全部 Room directed message。
func (s *RoomDirectedMessageStore) ReadMessages(conversationID string) ([]protocol.RoomDirectedMessageRecord, error) {
	rows, err := s.files.readJSONL(s.paths.RoomConversationMessagesPath(conversationID))
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.RoomDirectedMessageRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	messages := make([]protocol.RoomDirectedMessageRecord, 0, len(rows))
	for _, row := range rows {
		message := roomDirectedMessageFromRow(row)
		if strings.TrimSpace(message.MessageID) == "" {
			continue
		}
		messages = append(messages, message)
	}
	return messages, nil
}

// ReadContextMessages 读取对目标 agent 可见的近期 directed message。
func (s *RoomDirectedMessageStore) ReadContextMessages(conversationID string, agentID string) ([]protocol.RoomDirectedMessageRecord, error) {
	return s.ReadContextMessagesAfterCursor(conversationID, agentID, RoomDirectedMessageCursor{})
}

// ReadVisibleMessages 读取目标 agent 可见的全部 directed message，不裁剪上下文窗口。
func (s *RoomDirectedMessageStore) ReadVisibleMessages(conversationID string, agentID string) ([]protocol.RoomDirectedMessageRecord, error) {
	messages, err := s.ReadMessages(conversationID)
	if err != nil {
		return nil, err
	}
	targetAgentID := strings.TrimSpace(agentID)
	visible := make([]protocol.RoomDirectedMessageRecord, 0, len(messages))
	for _, message := range messages {
		if roomDirectedMessageVisibleToAgent(message, targetAgentID) {
			visible = append(visible, message)
		}
	}
	return visible, nil
}

// ReadContextMessagesAfterCursor 读取目标 agent cursor 之后可见的近期 directed message。
func (s *RoomDirectedMessageStore) ReadContextMessagesAfterCursor(
	conversationID string,
	agentID string,
	cursor RoomDirectedMessageCursor,
) ([]protocol.RoomDirectedMessageRecord, error) {
	visible, err := s.ReadVisibleMessages(conversationID, agentID)
	if err != nil {
		return nil, err
	}
	visible = roomDirectedMessagesAfterCursor(visible, cursor)
	if len(visible) > roomDirectedMessageContextLimit {
		visible = visible[len(visible)-roomDirectedMessageContextLimit:]
	}
	return visible, nil
}

// AppendMessageCursor 追加 Room directed message 消费位置控制行。
func (s *RoomDirectedMessageStore) AppendMessageCursor(cursor RoomDirectedMessageCursor) error {
	return s.files.appendJSONL(s.paths.RoomConversationMessageCursorsPath(cursor.ConversationID), roomDirectedMessageCursorToRow(cursor))
}

// ReadMessageCursor 读取目标 agent 最新 Room directed message 消费位置。
func (s *RoomDirectedMessageStore) ReadMessageCursor(conversationID string, agentID string) (RoomDirectedMessageCursor, bool, error) {
	if strings.TrimSpace(agentID) == "" {
		return RoomDirectedMessageCursor{}, false, nil
	}
	cursors, err := s.ReadMessageCursors(conversationID, agentID)
	if err != nil {
		return RoomDirectedMessageCursor{}, false, err
	}
	if len(cursors) == 0 {
		return RoomDirectedMessageCursor{}, false, nil
	}
	return cursors[0], true, nil
}

// ReadMessageCursors 读取每个 agent 最新的 Room directed message 消费位置。
func (s *RoomDirectedMessageStore) ReadMessageCursors(conversationID string, agentID string) ([]RoomDirectedMessageCursor, error) {
	rows, err := s.files.readJSONL(s.paths.RoomConversationMessageCursorsPath(conversationID))
	if errors.Is(err, os.ErrNotExist) {
		return []RoomDirectedMessageCursor{}, nil
	}
	if err != nil {
		return nil, err
	}
	targetAgentID := strings.TrimSpace(agentID)
	latestByAgentID := map[string]RoomDirectedMessageCursor{}
	for _, row := range rows {
		cursor := roomDirectedMessageCursorFromRow(row)
		cursorAgentID := strings.TrimSpace(cursor.AgentID)
		if cursorAgentID == "" ||
			strings.TrimSpace(cursor.ConversationID) != strings.TrimSpace(conversationID) ||
			strings.TrimSpace(cursor.LastMessageID) == "" ||
			(targetAgentID != "" && cursorAgentID != targetAgentID) {
			continue
		}
		latestByAgentID[cursorAgentID] = cursor
	}
	agentIDs := slices.Sorted(maps.Keys(latestByAgentID))
	cursors := make([]RoomDirectedMessageCursor, 0, len(agentIDs))
	for _, cursorAgentID := range agentIDs {
		cursors = append(cursors, latestByAgentID[cursorAgentID])
	}
	return cursors, nil
}

func roomDirectedMessageVisibleToAgent(message protocol.RoomDirectedMessageRecord, agentID string) bool {
	if strings.TrimSpace(agentID) == "" {
		return false
	}
	if containsRoomDirectedMessageAgent(message.Recipients, agentID) {
		return true
	}
	return message.ReplyRoute.Mode == protocol.RoomReplyRoutePrivate &&
		containsRoomDirectedMessageAgent(message.ReplyRoute.Recipients, agentID)
}

func roomDirectedMessagesAfterCursor(
	messages []protocol.RoomDirectedMessageRecord,
	cursor RoomDirectedMessageCursor,
) []protocol.RoomDirectedMessageRecord {
	if len(messages) == 0 {
		return nil
	}
	cursorMessageID := strings.TrimSpace(cursor.LastMessageID)
	if cursorMessageID != "" {
		for index, message := range messages {
			if strings.TrimSpace(message.MessageID) == cursorMessageID {
				return messages[index+1:]
			}
		}
	}
	if cursor.LastMessageTimestamp <= 0 {
		return messages
	}
	result := make([]protocol.RoomDirectedMessageRecord, 0, len(messages))
	for _, message := range messages {
		if message.Timestamp > cursor.LastMessageTimestamp {
			result = append(result, message)
		}
	}
	return result
}

func roomDirectedMessageToRow(message protocol.RoomDirectedMessageRecord) map[string]any {
	row := map[string]any{
		"message_id":      strings.TrimSpace(message.MessageID),
		"room_id":         strings.TrimSpace(message.RoomID),
		"conversation_id": strings.TrimSpace(message.ConversationID),
		"source_agent_id": strings.TrimSpace(message.SourceAgentID),
		"recipients":      slices.Clone(message.Recipients),
		"reply_route":     message.ReplyRoute,
		"timestamp":       message.Timestamp,
	}
	if message.WakePolicy != "" {
		row["wake_policy"] = string(message.WakePolicy)
	}
	if message.DelaySeconds > 0 {
		row["delay_seconds"] = message.DelaySeconds
	}
	if strings.TrimSpace(message.CorrelationID) != "" {
		row["correlation_id"] = strings.TrimSpace(message.CorrelationID)
	}
	if strings.TrimSpace(message.Content) != "" {
		row["content"] = message.Content
	}
	return row
}

func roomDirectedMessageCursorToRow(cursor RoomDirectedMessageCursor) map[string]any {
	return map[string]any{
		"room_id":                strings.TrimSpace(cursor.RoomID),
		"conversation_id":        strings.TrimSpace(cursor.ConversationID),
		"agent_id":               strings.TrimSpace(cursor.AgentID),
		"round_id":               strings.TrimSpace(cursor.RoundID),
		"last_message_id":        strings.TrimSpace(cursor.LastMessageID),
		"last_message_timestamp": cursor.LastMessageTimestamp,
		"timestamp":              cursor.Timestamp,
	}
}

func roomDirectedMessageCursorFromRow(row map[string]any) RoomDirectedMessageCursor {
	return RoomDirectedMessageCursor{
		RoomID:               stringFromAny(row["room_id"]),
		ConversationID:       stringFromAny(row["conversation_id"]),
		AgentID:              stringFromAny(row["agent_id"]),
		RoundID:              stringFromAny(row["round_id"]),
		LastMessageID:        stringFromAny(row["last_message_id"]),
		LastMessageTimestamp: int64FromAny(row["last_message_timestamp"]),
		Timestamp:            int64FromAny(row["timestamp"]),
	}
}

func roomDirectedMessageFromRow(row map[string]any) protocol.RoomDirectedMessageRecord {
	return protocol.RoomDirectedMessageRecord{
		MessageID:      stringFromAny(row["message_id"]),
		RoomID:         stringFromAny(row["room_id"]),
		ConversationID: stringFromAny(row["conversation_id"]),
		SourceAgentID:  stringFromAny(row["source_agent_id"]),
		Recipients:     stringSliceFromAny(row["recipients"]),
		Content:        stringFromAny(row["content"]),
		WakePolicy:     protocol.RoomWakePolicy(stringFromAny(row["wake_policy"])),
		ReplyRoute:     roomReplyRouteFromAny(row["reply_route"]),
		DelaySeconds:   int(int64FromAny(row["delay_seconds"])),
		CorrelationID:  stringFromAny(row["correlation_id"]),
		Timestamp:      int64FromAny(row["timestamp"]),
	}
}

func roomReplyRouteFromAny(value any) protocol.RoomReplyRoute {
	typed, ok := value.(map[string]any)
	if !ok {
		return protocol.RoomReplyRoute{}
	}
	route := protocol.RoomReplyRoute{
		Mode:       protocol.RoomReplyRouteMode(stringFromAny(typed["mode"])),
		Recipients: stringSliceFromAny(typed["recipients"]),
		WakePolicy: protocol.RoomWakePolicy(stringFromAny(typed["wake_policy"])),
	}
	next := roomReplyRouteFromAny(typed["next_reply_route"])
	if next.Mode != "" {
		route.NextReplyRoute = &next
	}
	return route
}

func containsRoomDirectedMessageAgent(items []string, value string) bool {
	return slices.ContainsFunc(items, func(item string) bool {
		return strings.TrimSpace(item) == value
	})
}
