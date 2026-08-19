// INPUT: Room directed message、host-only Goal collaboration attribution 与消费 cursor。
// OUTPUT: owner/conversation 隔离且可跨重启恢复的私域消息及 cursor。
// POS: Room 私域消息持久化真相源；Goal attribution 只参与宿主调度，不授权目标 Agent。
package workspace

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

// RoomDirectedMessageRecoveryRecord keeps the durable owner provenance needed
// when startup repair scans every Room conversation for an attributed wake
// whose handoff ledger write was interrupted.
type RoomDirectedMessageRecoveryRecord struct {
	OwnerPathSegment string
	OwnerUserID      string
	Message          protocol.RoomDirectedMessageRecord
}

// RoomDirectedMessageStore 负责 Room directed message 的 append-only 读写。
type RoomDirectedMessageStore struct {
	paths *Store
	files *SessionFileStore
	mu    sync.Mutex
}

// NewRoomDirectedMessageStore 创建 Room directed message 存储。
func NewRoomDirectedMessageStore(root string) *RoomDirectedMessageStore {
	paths := New(root)
	return &RoomDirectedMessageStore{
		paths: paths,
		files: newSessionFileStore(paths),
	}
}

// AppendMessage 追加一条 Room directed message。
func (s *RoomDirectedMessageStore) AppendMessage(
	ownerUserID string,
	message protocol.RoomDirectedMessageRecord,
) error {
	_, _, err := s.AppendMessageIfAbsent(ownerUserID, message)
	return err
}

// AppendMessageIfAbsent 按 message_id 持久接受一条私域消息。
// 同一逻辑工具调用重试时返回首次事实；相同 ID 的不同语义会 fail closed。
func (s *RoomDirectedMessageStore) AppendMessageIfAbsent(
	ownerUserID string,
	message protocol.RoomDirectedMessageRecord,
) (protocol.RoomDirectedMessageRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message.MessageID = strings.TrimSpace(message.MessageID)
	if message.MessageID == "" {
		return protocol.RoomDirectedMessageRecord{}, false, errors.New("message_id is required")
	}
	messages, err := s.readMessagesLocked(ownerUserID, message.ConversationID)
	if err != nil {
		return protocol.RoomDirectedMessageRecord{}, false, err
	}
	for _, existing := range messages {
		if strings.TrimSpace(existing.MessageID) != message.MessageID {
			continue
		}
		if !roomDirectedMessageSameIntent(existing, message) {
			return protocol.RoomDirectedMessageRecord{}, false, errors.New("room directed message idempotency conflict")
		}
		return existing, false, nil
	}
	row := roomDirectedMessageToRow(message)
	row["owner_user_id"] = strings.TrimSpace(ownerUserID)
	if err = s.files.appendRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationMessagesPath(ownerUserID, message.ConversationID),
		row,
	); err != nil {
		return protocol.RoomDirectedMessageRecord{}, false, err
	}
	return message, true, nil
}

// GoalCollaborationMessagesAll scans durable directed-message facts that can
// repair an interrupted message -> handoff write. It does not decide whether
// the recorded Goal revision is still current; the service layer owns that
// lifecycle check before recreating any wake.
func (s *RoomDirectedMessageStore) GoalCollaborationMessagesAll() ([]RoomDirectedMessageRecoveryRecord, error) {
	owners, err := listRoomOwnerPathSegments(s.paths.StateRoot)
	if err != nil {
		return nil, err
	}
	result := make([]RoomDirectedMessageRecoveryRecord, 0)
	for _, ownerPathSegment := range owners {
		roomRootPath := s.paths.RoomConversationRoot(ownerPathSegment)
		root, openErr := s.files.openRoomRoot(ownerPathSegment, false)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, openErr
		}
		entries, readDirErr := fs.ReadDir(root.FS(), ".")
		_ = root.Close()
		if readDirErr != nil {
			return nil, readDirErr
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			rows, readErr := s.files.readRoomJSONL(
				ownerPathSegment,
				filepath.Join(roomRootPath, entry.Name(), "directed_messages.jsonl"),
			)
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			if readErr != nil {
				return nil, readErr
			}
			for _, row := range rows {
				ownerUserID := ""
				if persistedOwnerUserID := stringFromAny(row["owner_user_id"]); persistedOwnerUserID != "" {
					var ok bool
					ownerUserID, ok = roomLedgerOwnerUserID(
						ownerPathSegment,
						persistedOwnerUserID,
					)
					if !ok {
						continue
					}
				}
				message := roomDirectedMessageFromRow(row)
				if strings.TrimSpace(message.MessageID) == "" ||
					protocol.NormalizeGoalCollaborationBinding(message.GoalCollaborationBinding) == nil ||
					(message.WakePolicy != protocol.RoomWakePolicyImmediate &&
						message.WakePolicy != protocol.RoomWakePolicyDelayed) ||
					filepath.Base(s.paths.RoomConversationDir(ownerPathSegment, message.ConversationID)) != entry.Name() {
					continue
				}
				result = append(result, RoomDirectedMessageRecoveryRecord{
					OwnerPathSegment: ownerPathSegment,
					OwnerUserID:      ownerUserID,
					Message:          message,
				})
			}
		}
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].Message.Timestamp != result[j].Message.Timestamp {
			return result[i].Message.Timestamp < result[j].Message.Timestamp
		}
		if result[i].OwnerUserID != result[j].OwnerUserID {
			return result[i].OwnerUserID < result[j].OwnerUserID
		}
		return result[i].Message.MessageID < result[j].Message.MessageID
	})
	return result, nil
}

// ReadMessages 读取指定对话的全部 Room directed message。
func (s *RoomDirectedMessageStore) ReadMessages(
	ownerUserID string,
	conversationID string,
) ([]protocol.RoomDirectedMessageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readMessagesLocked(ownerUserID, conversationID)
}

func (s *RoomDirectedMessageStore) readMessagesLocked(
	ownerUserID string,
	conversationID string,
) ([]protocol.RoomDirectedMessageRecord, error) {
	rows, err := s.files.readRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationMessagesPath(ownerUserID, conversationID),
	)
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.RoomDirectedMessageRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	messages := make([]protocol.RoomDirectedMessageRecord, 0, len(rows))
	conversationID = strings.TrimSpace(conversationID)
	for _, row := range rows {
		message := roomDirectedMessageFromRow(row)
		if strings.TrimSpace(message.MessageID) == "" {
			continue
		}
		// 文件目录是 conversation 事实源；行内字段不能把私域消息投影到
		// 同一 owner 的另一段对话。
		message.ConversationID = conversationID
		messages = append(messages, message)
	}
	return messages, nil
}

func roomDirectedMessageSameIntent(
	left protocol.RoomDirectedMessageRecord,
	right protocol.RoomDirectedMessageRecord,
) bool {
	left.Timestamp = 0
	right.Timestamp = 0
	left.GoalCollaborationBinding = protocol.NormalizeGoalCollaborationBinding(left.GoalCollaborationBinding)
	right.GoalCollaborationBinding = protocol.NormalizeGoalCollaborationBinding(right.GoalCollaborationBinding)
	return reflect.DeepEqual(left, right)
}

// ReadContextMessages 读取对目标 agent 可见的近期 directed message。
func (s *RoomDirectedMessageStore) ReadContextMessages(
	ownerUserID string,
	conversationID string,
	agentID string,
) ([]protocol.RoomDirectedMessageRecord, error) {
	return s.ReadContextMessagesAfterCursor(ownerUserID, conversationID, agentID, RoomDirectedMessageCursor{})
}

// ReadVisibleMessages 读取目标 agent 可见的全部 directed message，不裁剪上下文窗口。
func (s *RoomDirectedMessageStore) ReadVisibleMessages(
	ownerUserID string,
	conversationID string,
	agentID string,
) ([]protocol.RoomDirectedMessageRecord, error) {
	messages, err := s.ReadMessages(ownerUserID, conversationID)
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
	ownerUserID string,
	conversationID string,
	agentID string,
	cursor RoomDirectedMessageCursor,
) ([]protocol.RoomDirectedMessageRecord, error) {
	visible, err := s.ReadVisibleMessages(ownerUserID, conversationID, agentID)
	if err != nil {
		return nil, err
	}
	visible = roomDirectedMessagesAfterCursor(visible, cursor)
	return visible, nil
}

// ReadContextMessagesThrough 读取 cursor 之后、不超过指定触发消息的私域上下文。
func (s *RoomDirectedMessageStore) ReadContextMessagesThrough(
	ownerUserID string,
	conversationID string,
	agentID string,
	cursor RoomDirectedMessageCursor,
	lastMessageID string,
) ([]protocol.RoomDirectedMessageRecord, error) {
	visible, err := s.ReadVisibleMessages(ownerUserID, conversationID, agentID)
	if err != nil {
		return nil, err
	}
	visible = roomDirectedMessagesAfterCursor(visible, cursor)
	boundary := strings.TrimSpace(lastMessageID)
	if boundary != "" {
		for index, message := range visible {
			if strings.TrimSpace(message.MessageID) == boundary {
				visible = visible[:index+1]
				break
			}
		}
	}
	return visible, nil
}

// AppendMessageCursor 追加 Room directed message 消费位置控制行。
func (s *RoomDirectedMessageStore) AppendMessageCursor(
	ownerUserID string,
	cursor RoomDirectedMessageCursor,
) error {
	return s.files.appendRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationMessageCursorsPath(ownerUserID, cursor.ConversationID),
		roomDirectedMessageCursorToRow(cursor),
	)
}

// ReadMessageCursor 读取目标 agent 最新 Room directed message 消费位置。
func (s *RoomDirectedMessageStore) ReadMessageCursor(
	ownerUserID string,
	conversationID string,
	agentID string,
) (RoomDirectedMessageCursor, bool, error) {
	if strings.TrimSpace(agentID) == "" {
		return RoomDirectedMessageCursor{}, false, nil
	}
	cursors, err := s.ReadMessageCursors(ownerUserID, conversationID, agentID)
	if err != nil {
		return RoomDirectedMessageCursor{}, false, err
	}
	if len(cursors) == 0 {
		return RoomDirectedMessageCursor{}, false, nil
	}
	return cursors[0], true, nil
}

// ReadMessageCursors 读取每个 agent 最新的 Room directed message 消费位置。
func (s *RoomDirectedMessageStore) ReadMessageCursors(
	ownerUserID string,
	conversationID string,
	agentID string,
) ([]RoomDirectedMessageCursor, error) {
	rows, err := s.files.readRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationMessageCursorsPath(ownerUserID, conversationID),
	)
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
		cursor.ConversationID = strings.TrimSpace(conversationID)
		cursorAgentID := strings.TrimSpace(cursor.AgentID)
		if cursorAgentID == "" ||
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
	if len(message.WakeTargets) > 0 {
		row["wake_targets"] = slices.Clone(message.WakeTargets)
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
	if strings.TrimSpace(message.RootRoundID) != "" {
		row["root_round_id"] = strings.TrimSpace(message.RootRoundID)
	}
	if strings.TrimSpace(message.CausedByRoundID) != "" {
		row["caused_by_round_id"] = strings.TrimSpace(message.CausedByRoundID)
	}
	if binding := protocol.NormalizeGoalCollaborationBinding(message.GoalCollaborationBinding); binding != nil {
		row["goal_collaboration_binding"] = binding
	}
	if message.HopIndex > 0 {
		row["hop_index"] = message.HopIndex
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
		LastMessageTimestamp: protocol.Int64FromAny(row["last_message_timestamp"]),
		Timestamp:            protocol.Int64FromAny(row["timestamp"]),
	}
}

func roomDirectedMessageFromRow(row map[string]any) protocol.RoomDirectedMessageRecord {
	return protocol.RoomDirectedMessageRecord{
		MessageID:                stringFromAny(row["message_id"]),
		RoomID:                   stringFromAny(row["room_id"]),
		ConversationID:           stringFromAny(row["conversation_id"]),
		SourceAgentID:            stringFromAny(row["source_agent_id"]),
		Recipients:               stringSliceFromAny(row["recipients"]),
		WakeTargets:              stringSliceFromAny(row["wake_targets"]),
		Content:                  stringFromAny(row["content"]),
		WakePolicy:               protocol.RoomWakePolicy(stringFromAny(row["wake_policy"])),
		ReplyRoute:               roomReplyRouteFromAny(row["reply_route"]),
		DelaySeconds:             int(protocol.Int64FromAny(row["delay_seconds"])),
		CorrelationID:            stringFromAny(row["correlation_id"]),
		RootRoundID:              stringFromAny(row["root_round_id"]),
		CausedByRoundID:          stringFromAny(row["caused_by_round_id"]),
		GoalCollaborationBinding: goalCollaborationBindingFromAny(row["goal_collaboration_binding"]),
		HopIndex:                 int(protocol.Int64FromAny(row["hop_index"])),
		Timestamp:                protocol.Int64FromAny(row["timestamp"]),
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
