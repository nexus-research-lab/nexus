package workspace

import (
	"errors"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const overlayKindTranscriptRef = "transcript_ref"

// RoomHistoryStore 负责 Room 共享历史读写。
// 共享层只保存两类数据：
// 1. Room 自己的 inline overlay（用户消息、synthetic result 等）。
// 2. 指向成员 transcript 的引用行，真正正文从 transcript 投影恢复。
type RoomHistoryStore struct {
	paths        *Store
	files        *SessionFileStore
	agentHistory *AgentHistoryStore
}

// NewRoomHistoryStore 创建 Room 共享历史门面。
func NewRoomHistoryStore(root string) *RoomHistoryStore {
	return &RoomHistoryStore{
		paths:        New(root),
		files:        NewSessionFileStore(root),
		agentHistory: NewAgentHistoryStore(root),
	}
}

// AppendInlineMessage 追加一条 Room inline overlay。
func (s *RoomHistoryStore) AppendInlineMessage(conversationID string, message protocol.Message) error {
	return s.files.appendJSONL(s.paths.RoomConversationOverlayPath(conversationID), message)
}

// AppendTranscriptReference 追加一条 transcript 引用。
// 当引用条件不完整时，退回成 inline overlay，避免共享历史丢数据。
func (s *RoomHistoryStore) AppendTranscriptReference(
	conversationID string,
	workspacePath string,
	privateSessionKey string,
	message protocol.Message,
) error {
	row := buildRoomTranscriptReference(message, workspacePath, privateSessionKey)
	if row == nil {
		return s.AppendInlineMessage(conversationID, message)
	}
	return s.files.appendJSONL(s.paths.RoomConversationOverlayPath(conversationID), row)
}

// ReadMessages 读取 Room 共享历史。
func (s *RoomHistoryStore) ReadMessages(
	conversationID string,
	activeRoundIDs []string,
) ([]protocol.Message, error) {
	rows, err := s.readResolvedRows(conversationID)
	if err != nil {
		return nil, err
	}
	return normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs)), nil
}

// ReadMessagesPage 按 round 读取 Room 共享历史分页。
func (s *RoomHistoryStore) ReadMessagesPage(
	conversationID string,
	activeRoundIDs []string,
	limit int,
	beforeRoundID string,
	beforeRoundTimestamp int64,
) (protocol.MessagePage, error) {
	rows, err := s.readResolvedRows(conversationID)
	if err != nil {
		return protocol.MessagePage{}, err
	}
	normalizedRows := normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs))
	return paginateNormalizedHistoryRows(
		normalizedRows,
		limit,
		beforeRoundID,
		beforeRoundTimestamp,
		true,
	), nil
}

func (s *RoomHistoryStore) readResolvedRows(conversationID string) ([]protocol.Message, error) {
	rows, err := s.files.readJSONL(s.paths.RoomConversationOverlayPath(conversationID))
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.Message{}, nil
	}
	if err != nil {
		return nil, err
	}

	transcriptRowsByMessageID := make(map[string]map[string]protocol.Message)
	resolved := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(stringFromAny(row[overlayKindField])) != overlayKindTranscriptRef {
			resolved = append(resolved, protocol.Message(row))
			continue
		}
		messageValue, ok, resolveErr := s.resolveTranscriptReference(
			protocol.Message(row),
			transcriptRowsByMessageID,
		)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if ok {
			resolved = append(resolved, messageValue)
		}
	}
	return resolved, nil
}

func (s *RoomHistoryStore) resolveTranscriptReference(
	row protocol.Message,
	cache map[string]map[string]protocol.Message,
) (protocol.Message, bool, error) {
	workspacePath := strings.TrimSpace(stringFromAny(row["workspace_path"]))
	privateSessionKey := strings.TrimSpace(stringFromAny(row["private_session_key"]))
	agentID := strings.TrimSpace(stringFromAny(row["agent_id"]))
	sessionID := strings.TrimSpace(stringFromAny(row["session_id"]))
	messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
	if workspacePath == "" || privateSessionKey == "" || agentID == "" || sessionID == "" || messageID == "" {
		return nil, false, nil
	}

	cacheKey := buildRoomTranscriptCacheKey(workspacePath, privateSessionKey, agentID, sessionID)
	messageIndex, exists := cache[cacheKey]
	if !exists {
		_, roundMarkers, err := s.agentHistory.readOverlayRowsAndMarkers(workspacePath, privateSessionKey)
		if err != nil {
			return nil, false, err
		}
		transcriptRows, err := s.agentHistory.readTranscriptMessages(
			workspacePath,
			privateSessionKey,
			agentID,
			sessionID,
			roundMarkers,
		)
		if errors.Is(err, os.ErrNotExist) {
			cache[cacheKey] = map[string]protocol.Message{}
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		messageIndex = indexRoomTranscriptMessages(transcriptRows)
		cache[cacheKey] = messageIndex
	}

	transcriptMessage, ok := messageIndex[messageID]
	if !ok {
		return nil, false, nil
	}

	resolved := protocol.Clone(transcriptMessage)
	overrideRoomTranscriptFields(resolved, row)
	return resolved, true, nil
}

func buildRoomTranscriptReference(
	message protocol.Message,
	workspacePath string,
	privateSessionKey string,
) map[string]any {
	if protocol.MessageRole(message) != "assistant" {
		return nil
	}
	sessionID := strings.TrimSpace(stringFromAny(message["session_id"]))
	messageID := strings.TrimSpace(stringFromAny(message["message_id"]))
	if sessionID == "" || messageID == "" || strings.TrimSpace(workspacePath) == "" || strings.TrimSpace(privateSessionKey) == "" {
		return nil
	}

	row := map[string]any{
		overlayKindField:      overlayKindTranscriptRef,
		"message_id":          messageID,
		"conversation_id":     strings.TrimSpace(stringFromAny(message["conversation_id"])),
		"agent_id":            strings.TrimSpace(stringFromAny(message["agent_id"])),
		"round_id":            strings.TrimSpace(stringFromAny(message["round_id"])),
		"session_id":          sessionID,
		"timestamp":           messageTimestamp(message),
		"workspace_path":      strings.TrimSpace(workspacePath),
		"private_session_key": strings.TrimSpace(privateSessionKey),
	}
	return row
}

func buildRoomTranscriptCacheKey(
	workspacePath string,
	privateSessionKey string,
	agentID string,
	sessionID string,
) string {
	return strings.Join([]string{workspacePath, privateSessionKey, agentID, sessionID}, "\x00")
}

func indexRoomTranscriptMessages(rows []protocol.Message) map[string]protocol.Message {
	result := make(map[string]protocol.Message, len(rows))
	for _, row := range rows {
		messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
		if messageID == "" {
			continue
		}
		result[messageID] = protocol.Clone(row)
	}
	return result
}

func overrideRoomTranscriptFields(target protocol.Message, source protocol.Message) {
	for _, key := range []string{
		"message_id",
		"conversation_id",
		"agent_id",
		"round_id",
	} {
		if value := strings.TrimSpace(stringFromAny(source[key])); value != "" {
			target[key] = value
		}
	}
	if timestamp := messageTimestamp(source); timestamp > 0 {
		target["timestamp"] = timestamp
	}
	if sessionID := strings.TrimSpace(stringFromAny(source["session_id"])); sessionID != "" {
		target["session_id"] = sessionID
	}
}
