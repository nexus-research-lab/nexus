// INPUT: Room inline overlay、成员 transcript 引用与 Agent 执行身份。
// OUTPUT: 保留 root/agent round、parent 关系与 host-owned public handoff reply 因果的 Room 共享历史。
// POS: Room 公区历史在 JSONL overlay 与成员 transcript 之间的持久化边界。
package workspace

import (
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const overlayKindTranscriptRef = "transcript_ref"

// RoomTranscriptReference 是删除 Room 产物所需的稳定 transcript 引用。
type RoomTranscriptReference struct {
	AgentID           string `json:"agent_id"`
	ConversationID    string `json:"conversation_id"`
	PrivateSessionKey string `json:"private_session_key"`
	SessionID         string `json:"session_id"`
	WorkspacePath     string `json:"workspace_path"`
}

// RoomHistoryStore 负责 Room 共享历史读写。
// 共享层只保存两类数据：
// 1. Room 自己的 inline overlay（用户消息、synthetic result 等）。
// 2. 指向成员 transcript 的引用行，真正正文从 transcript 投影恢复。
type RoomHistoryStore struct {
	paths        *Store
	files        *SessionFileStore
	agentHistory *AgentHistoryStore
	countMu      sync.Mutex
	countByKey   map[string]roomHistoryCountSnapshot
}

type roomHistoryCountSnapshot struct {
	fileSize       int64
	modifiedUnixNS int64
	count          int
}

// NewRoomHistoryStore 创建 Room 共享历史门面。
func NewRoomHistoryStore(root string) *RoomHistoryStore {
	paths := New(root)
	return &RoomHistoryStore{
		paths:        paths,
		files:        newSessionFileStore(paths),
		agentHistory: NewAgentHistoryStore(root),
		countByKey:   make(map[string]roomHistoryCountSnapshot),
	}
}

// AppendInlineMessage 追加一条 Room inline overlay。
func (s *RoomHistoryStore) AppendInlineMessage(
	ownerUserID string,
	conversationID string,
	message protocol.Message,
) error {
	message = protocol.Clone(message)
	message["conversation_id"] = strings.TrimSpace(conversationID)
	return s.files.appendRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
		message,
	)
}

// AppendTranscriptReference 追加一条 transcript 引用。
// 当引用条件不完整时，退回成 inline overlay，避免共享历史丢数据。
func (s *RoomHistoryStore) AppendTranscriptReference(
	ownerUserID string,
	conversationID string,
	workspacePath string,
	privateSessionKey string,
	message protocol.Message,
) error {
	row := buildRoomTranscriptReference(message, workspacePath, privateSessionKey)
	if row == nil || !s.paths.workspacePathIsConfinedForOwner(ownerUserID, workspacePath) {
		return s.AppendInlineMessage(ownerUserID, conversationID, message)
	}
	row["conversation_id"] = strings.TrimSpace(conversationID)
	return s.files.appendRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
		row,
	)
}

// ReadMessages 读取 Room 共享历史。
func (s *RoomHistoryStore) ReadMessages(
	ownerUserID string,
	conversationID string,
	activeRoundIDs []string,
) ([]protocol.Message, error) {
	rows, err := s.readResolvedRows(ownerUserID, conversationID)
	if err != nil {
		return nil, err
	}
	return normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs)), nil
}

// MessageCount 返回 Room 共享历史的可见消息数。
// 计数以 JSONL ledger 为真相，按文件长度缓存；只有 ledger 变化时才重新投影。
func (s *RoomHistoryStore) MessageCount(ownerUserID string, conversationID string) (int, error) {
	path := s.paths.RoomConversationOverlayPath(ownerUserID, conversationID)
	parent, name, err := s.files.openRoomFileParent(ownerUserID, path, false)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	info, err := parent.Lstat(name)
	parent.Close()
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cacheKey := strings.Join([]string{
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(conversationID),
	}, "\x00")
	s.countMu.Lock()
	cached, ok := s.countByKey[cacheKey]
	s.countMu.Unlock()
	if ok && cached.fileSize == info.Size() && cached.modifiedUnixNS == info.ModTime().UnixNano() {
		return cached.count, nil
	}
	rows, err := s.ReadMessages(ownerUserID, conversationID, nil)
	if err != nil {
		return 0, err
	}
	parent, name, err = s.files.openRoomFileParent(ownerUserID, path, false)
	if err != nil {
		return 0, err
	}
	latestInfo, err := parent.Lstat(name)
	parent.Close()
	if err != nil {
		return 0, err
	}
	if latestInfo.Size() != info.Size() || latestInfo.ModTime() != info.ModTime() {
		rows, err = s.ReadMessages(ownerUserID, conversationID, nil)
		if err != nil {
			return 0, err
		}
		parent, name, err = s.files.openRoomFileParent(ownerUserID, path, false)
		if err != nil {
			return 0, err
		}
		info, err = parent.Lstat(name)
		parent.Close()
		if err != nil {
			return 0, err
		}
	}
	s.countMu.Lock()
	s.countByKey[cacheKey] = roomHistoryCountSnapshot{
		fileSize:       info.Size(),
		modifiedUnixNS: info.ModTime().UnixNano(),
		count:          len(rows),
	}
	s.countMu.Unlock()
	return len(rows), nil
}

// ListTranscriptReferences 在共享 overlay 删除前抽取所有历史 transcript 引用。
func (s *RoomHistoryStore) ListTranscriptReferences(
	ownerUserID string,
	conversationID string,
) ([]RoomTranscriptReference, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	conversationID = strings.TrimSpace(conversationID)
	rows, err := s.files.readRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
	)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	result := make([]RoomTranscriptReference, 0)
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) != overlayKindTranscriptRef {
			continue
		}
		item := RoomTranscriptReference{
			AgentID:           stringFromAny(row["agent_id"]),
			ConversationID:    conversationID,
			PrivateSessionKey: stringFromAny(row["private_session_key"]),
			SessionID:         strings.ToLower(stringFromAny(row["session_id"])),
			WorkspacePath:     stringFromAny(row["workspace_path"]),
		}
		if item.AgentID == "" || item.PrivateSessionKey == "" ||
			!IsTranscriptSessionID(item.SessionID) ||
			!s.paths.workspacePathIsConfinedForOwner(ownerUserID, item.WorkspacePath) {
			continue
		}
		key := strings.Join([]string{
			item.AgentID,
			item.PrivateSessionKey,
			item.SessionID,
			item.WorkspacePath,
		}, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result, nil
}

// ReadMessagesPage 按 round 读取 Room 共享历史分页。
func (s *RoomHistoryStore) ReadMessagesPage(
	ownerUserID string,
	conversationID string,
	activeRoundIDs []string,
	limit int,
	beforeRoundID string,
	beforeRoundTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
) (protocol.MessagePage, error) {
	rows, err := s.readResolvedRows(ownerUserID, conversationID)
	if err != nil {
		return protocol.MessagePage{}, err
	}
	normalizedRows := normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs))
	if strings.TrimSpace(aroundRoundID) != "" {
		return paginateNormalizedHistoryRowsAround(
			normalizedRows,
			aroundRoundID,
			aroundLimit,
			true,
		), nil
	}
	return paginateNormalizedHistoryRows(
		normalizedRows,
		limit,
		beforeRoundID,
		beforeRoundTimestamp,
		true,
	), nil
}

func (s *RoomHistoryStore) readResolvedRows(ownerUserID string, conversationID string) ([]protocol.Message, error) {
	rows, err := s.files.readRoomJSONL(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
	)
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.Message{}, nil
	}
	if err != nil {
		return nil, err
	}

	transcriptRowsByMessageID := make(map[string]map[string]protocol.Message)
	resolved := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) != overlayKindTranscriptRef {
			message := protocol.Message(row)
			message["conversation_id"] = strings.TrimSpace(conversationID)
			resolved = append(resolved, message)
			continue
		}
		messageValue, ok, resolveErr := s.resolveTranscriptReference(
			ownerUserID,
			protocol.Message(row),
			transcriptRowsByMessageID,
		)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if ok {
			messageValue["conversation_id"] = strings.TrimSpace(conversationID)
			resolved = append(resolved, messageValue)
		}
	}
	return resolved, nil
}

func (s *RoomHistoryStore) resolveTranscriptReference(
	ownerUserID string,
	row protocol.Message,
	cache map[string]map[string]protocol.Message,
) (protocol.Message, bool, error) {
	workspacePath := stringFromAny(row["workspace_path"])
	privateSessionKey := stringFromAny(row["private_session_key"])
	agentID := stringFromAny(row["agent_id"])
	sessionID := stringFromAny(row["session_id"])
	messageID := stringFromAny(row["message_id"])
	if workspacePath == "" || privateSessionKey == "" || agentID == "" || sessionID == "" || messageID == "" {
		return nil, false, nil
	}
	if !s.paths.workspacePathIsConfinedForOwner(ownerUserID, workspacePath) {
		return nil, false, nil
	}

	cacheKey := buildRoomTranscriptCacheKey(workspacePath, privateSessionKey, agentID, sessionID)
	messageIndex, exists := cache[cacheKey]
	if !exists {
		ownerHistory := s.agentHistory.ForOwner(ownerUserID)
		_, roundMarkers, err := ownerHistory.readOverlayRowsAndMarkers(
			workspacePath,
			privateSessionKey,
		)
		if err != nil {
			return nil, false, err
		}
		transcriptRows, err := ownerHistory.readTranscriptMessages(
			workspacePath,
			privateSessionKey,
			agentID,
			sessionID,
			roundMarkers,
			"",
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
	workspacePath = strings.TrimSpace(workspacePath)
	privateSessionKey = strings.TrimSpace(privateSessionKey)
	sessionID := stringFromAny(message["session_id"])
	messageID := stringFromAny(message["message_id"])
	if sessionID == "" || messageID == "" || workspacePath == "" || privateSessionKey == "" {
		return nil
	}

	row := map[string]any{
		overlayKindField:      overlayKindTranscriptRef,
		"message_id":          messageID,
		"conversation_id":     stringFromAny(message["conversation_id"]),
		"agent_id":            stringFromAny(message["agent_id"]),
		"round_id":            stringFromAny(message["round_id"]),
		"session_id":          sessionID,
		"timestamp":           messageTimestamp(message),
		"workspace_path":      workspacePath,
		"private_session_key": privateSessionKey,
	}
	copyRoomTranscriptReferenceIdentity(row, message, "agent_round_id")
	copyRoomTranscriptReferenceIdentity(row, message, "parent_id")
	copyRoomTranscriptReferenceValue(row, message, "agent_mentions")
	copyRoomTranscriptReferenceValue(row, message, "handoff_reply")
	copyRoomTranscriptReferenceValue(row, message, "display_order")
	return row
}

func copyRoomTranscriptReferenceIdentity(target map[string]any, source protocol.Message, key string) {
	if value := stringFromAny(source[key]); value != "" {
		target[key] = value
	}
}

func copyRoomTranscriptReferenceValue(target map[string]any, source protocol.Message, key string) {
	if value, exists := source[key]; exists && value != nil {
		target[key] = value
	}
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
		messageID := stringFromAny(row["message_id"])
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
		"agent_round_id",
		"parent_id",
	} {
		if value := stringFromAny(source[key]); value != "" {
			target[key] = value
		}
	}
	for _, key := range []string{"agent_mentions", "handoff_reply", "display_order"} {
		if value, exists := source[key]; exists && value != nil {
			target[key] = value
		}
	}
	if timestamp := messageTimestamp(source); timestamp > 0 {
		target["timestamp"] = timestamp
	}
	if sessionID := stringFromAny(source["session_id"]); sessionID != "" {
		target["session_id"] = sessionID
	}
}
