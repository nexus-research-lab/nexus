// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：session_files.go
// @Date   ：2026/04/11 00:02:00
// @Author ：leemysw
// 2026/04/11 00:02:00   Create
// =====================================================

package workspace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
)

// SessionFileStore 负责 workspace 侧会话文件读写。
type SessionFileStore struct {
	paths *Store
}

// NewSessionFileStore 创建文件存储门面。
func NewSessionFileStore(root string) *SessionFileStore {
	return &SessionFileStore{
		paths: New(root),
	}
}

// ListSessions 读取某个 workspace 下的全部文件会话。
func (s *SessionFileStore) ListSessions(workspacePath string) ([]sessionmodel.Session, error) {
	sessionRoot := filepath.Join(workspacePath, ".agents", "sessions")
	entries, err := os.ReadDir(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []sessionmodel.Session{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]sessionmodel.Session, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(sessionRoot, entry.Name(), "meta.json")
		item, loadErr := s.readSessionMeta(metaPath)
		if errors.Is(loadErr, os.ErrNotExist) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].LastActivity.After(result[j].LastActivity)
	})
	return result, nil
}

// FindSession 在多个 workspace 中定位单个 session。
func (s *SessionFileStore) FindSession(workspacePaths []string, sessionKey string) (*sessionmodel.Session, string, error) {
	for _, workspacePath := range workspacePaths {
		metaPath := s.paths.SessionMetaPath(workspacePath, sessionKey)
		item, err := s.readSessionMeta(metaPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		return &item, workspacePath, nil
	}
	return nil, "", nil
}

// UpsertSession 创建或更新 session meta。
func (s *SessionFileStore) UpsertSession(workspacePath string, item sessionmodel.Session) (*sessionmodel.Session, error) {
	metaPath := s.paths.SessionMetaPath(workspacePath, item.SessionKey)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return nil, err
	}

	// 这里直接以 Go 模型作为 meta 真相源，避免再复制一套弱类型结构。
	payload, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(metaPath, payload, 0o644); err != nil {
		return nil, err
	}
	created, _, err := s.FindSession([]string{workspacePath}, item.SessionKey)
	return created, err
}

// DeleteSession 删除整个 session 目录。
func (s *SessionFileStore) DeleteSession(workspacePath string, sessionKey string) (bool, error) {
	sessionDir := s.paths.SessionDir(workspacePath, sessionKey)
	if _, err := os.Stat(sessionDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(sessionDir); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteRoomConversation 删除 Room 对话共享目录。
func (s *SessionFileStore) DeleteRoomConversation(conversationID string) (bool, error) {
	conversationDir := s.paths.RoomConversationDir(conversationID)
	if _, err := os.Stat(conversationDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(conversationDir); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SessionFileStore) readSessionMeta(metaPath string) (sessionmodel.Session, error) {
	payload, err := os.ReadFile(metaPath)
	if err != nil {
		return sessionmodel.Session{}, err
	}
	var item sessionmodel.Session
	if err = json.Unmarshal(payload, &item); err != nil {
		return sessionmodel.Session{}, err
	}
	if item.Options == nil {
		item.Options = map[string]any{}
	}
	if item.Title == "" {
		item.Title = "New Chat"
	}
	if item.ChannelType == "" {
		item.ChannelType = "websocket"
	}
	if item.ChatType == "" {
		item.ChatType = "dm"
	}
	item.IsActive = item.Status == "" || item.Status == "active"
	if item.Status == "" {
		item.Status = "active"
	}
	if item.LastActivity.IsZero() {
		item.LastActivity = item.CreatedAt
	}
	return item, nil
}

func (s *SessionFileStore) readMessagesFromPath(path string) ([]sessionmodel.Message, error) {
	rows, err := s.readJSONL(path)
	if err != nil {
		return nil, err
	}
	result := make([]sessionmodel.Message, 0, len(rows))
	for _, row := range rows {
		result = append(result, sessionmodel.Message(row))
	}
	return result, nil
}

func (s *SessionFileStore) appendJSONL(path string, row map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	payload, err := json.Marshal(row)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(file, "%s\n", payload); err != nil {
		return err
	}
	return nil
}

func (s *SessionFileStore) replaceJSONL(path string, rows []map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tempFile, err := os.CreateTemp(filepath.Dir(path), ".jsonl-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		_ = tempFile.Close()
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	writer := bufio.NewWriter(tempFile)
	for _, row := range rows {
		payload, marshalErr := json.Marshal(row)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = fmt.Fprintf(writer, "%s\n", payload); err != nil {
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}
	if err = tempFile.Close(); err != nil {
		return err
	}
	if err = os.Rename(tempPath, path); err != nil {
		return err
	}
	cleanupTemp = false
	return nil
}

func (s *SessionFileStore) readJSONL(path string) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewScanner(file)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	rows := make([]map[string]any, 0)
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		normalized, ok := normalizeDecodedJSONValue(item).(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, normalized)
	}
	return rows, reader.Err()
}

func compactMessages(rows []sessionmodel.Message) []sessionmodel.Message {
	latestByID := make(map[string]sessionmodel.Message, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
		if messageID == "" {
			continue
		}
		if current, exists := latestByID[messageID]; exists {
			latestByID[messageID] = mergeCompactedMessage(current, cloneMessage(row))
			continue
		}
		if _, exists := latestByID[messageID]; !exists {
			order = append(order, messageID)
		}
		latestByID[messageID] = cloneMessage(row)
	}

	compacted := make([]sessionmodel.Message, 0, len(order))
	for _, messageID := range order {
		compacted = append(compacted, latestByID[messageID])
	}
	sortHistoryRows(compacted)
	return compacted
}

func mergeCompactedMessage(current sessionmodel.Message, next sessionmodel.Message) sessionmodel.Message {
	if strings.TrimSpace(stringFromAny(current["role"])) != "assistant" || strings.TrimSpace(stringFromAny(next["role"])) != "assistant" {
		return next
	}
	return mergeAssistantSnapshots(current, next)
}

func mergeAssistantSnapshots(current sessionmodel.Message, next sessionmodel.Message) sessionmodel.Message {
	merged := cloneMessage(current)

	// assistant 快照属于同一条消息的增量物化，身份字段一旦建立就不应被后续快照改写。
	identityKeys := []string{
		"message_id",
		"session_key",
		"room_id",
		"conversation_id",
		"agent_id",
		"round_id",
		"parent_id",
		"session_id",
		"role",
	}
	for _, key := range identityKeys {
		if strings.TrimSpace(stringFromAny(merged[key])) == "" && strings.TrimSpace(stringFromAny(next[key])) != "" {
			merged[key] = next[key]
		}
	}

	if content, ok := mergeAssistantContentBlocks(merged["content"], next["content"]); ok {
		merged["content"] = content
	} else if next["content"] != nil {
		merged["content"] = next["content"]
	}
	if value := strings.TrimSpace(stringFromAny(next["model"])); value != "" {
		merged["model"] = value
	}
	if value := strings.TrimSpace(stringFromAny(next["stop_reason"])); value != "" {
		merged["stop_reason"] = value
	}
	if usage := normalizeMapValue(next["usage"]); len(usage) > 0 {
		merged["usage"] = usage
	}
	if boolFromAny(current["is_complete"]) || boolFromAny(next["is_complete"]) {
		merged["is_complete"] = true
	}
	if status := strings.TrimSpace(stringFromAny(next["stream_status"])); status != "" {
		merged["stream_status"] = status
	}
	if ts := messageTimestamp(next); ts >= messageTimestamp(current) {
		merged["timestamp"] = next["timestamp"]
	}

	// 其它非身份字段以后者为准，避免成本、终止原因、补充元数据被旧快照覆盖。
	for key, value := range next {
		switch key {
		case "content",
			"message_id",
			"session_key",
			"room_id",
			"conversation_id",
			"agent_id",
			"round_id",
			"parent_id",
			"session_id",
			"role",
			"model",
			"stop_reason",
			"usage",
			"is_complete",
			"stream_status",
			"timestamp":
			continue
		default:
			merged[key] = value
		}
	}
	return merged
}

func mergeAssistantContentBlocks(current any, next any) ([]map[string]any, bool) {
	result := normalizeMessageContentBlocks(current)
	incoming := normalizeMessageContentBlocks(next)
	if result == nil && incoming == nil {
		return nil, false
	}
	if len(result) == 0 {
		return incoming, true
	}
	if len(incoming) == 0 {
		return result, true
	}
	for _, block := range incoming {
		result = upsertAssistantContentBlock(result, block)
	}
	return result, true
}

func normalizeMessageContentBlocks(raw any) []map[string]any {
	switch typed := raw.(type) {
	case []map[string]any:
		return cloneMessageContentBlocks(typed)
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			payload, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, cloneMessageMap(payload))
		}
		return result
	default:
		return nil
	}
}

func cloneMessageContentBlocks(blocks []map[string]any) []map[string]any {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, cloneMessageMap(block))
	}
	return result
}

func cloneMessageMap(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = value
	}
	return result
}

func upsertAssistantContentBlock(blocks []map[string]any, incoming map[string]any) []map[string]any {
	block := cloneMessageMap(incoming)
	if len(block) == 0 {
		return blocks
	}
	incomingType := strings.TrimSpace(stringFromAny(block["type"]))
	for index, current := range blocks {
		currentType := strings.TrimSpace(stringFromAny(current["type"]))
		if currentType != incomingType {
			continue
		}
		switch incomingType {
		case "thinking":
			blocks[index] = block
			return blocks
		case "text":
			blocks[index] = block
			return blocks
		case "tool_use":
			if strings.TrimSpace(stringFromAny(current["id"])) == strings.TrimSpace(stringFromAny(block["id"])) {
				blocks[index] = block
				return blocks
			}
		case "tool_result":
			if strings.TrimSpace(stringFromAny(current["tool_use_id"])) == strings.TrimSpace(stringFromAny(block["tool_use_id"])) {
				blocks[index] = block
				return blocks
			}
		case "task_progress":
			if strings.TrimSpace(stringFromAny(current["task_id"])) == strings.TrimSpace(stringFromAny(block["task_id"])) {
				blocks[index] = block
				return blocks
			}
		default:
			blocks[index] = block
			return blocks
		}
	}
	return append(blocks, block)
}

func normalizeMapValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMessageMap(typed)
	default:
		return nil
	}
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return false
	}
}

func messageTimestamp(row sessionmodel.Message) int64 {
	value := row["timestamp"]
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func floatFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

func normalizeDecodedJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = normalizeDecodedJSONValue(item)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, normalizeDecodedJSONValue(item))
		}
		return result
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed)
		}
		return typed
	default:
		return value
	}
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringPointer(value string) *string {
	copyValue := value
	return &copyValue
}

func intPointer(value int) *int {
	copyValue := value
	return &copyValue
}

func floatPointer(value float64) *float64 {
	copyValue := value
	return &copyValue
}
