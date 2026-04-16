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
	"github.com/nexus-research-lab/nexus-core/internal/sessiondomain"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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

// RoomConversationMessagePath 返回 Room 对话共享日志路径。
func (s *SessionFileStore) RoomConversationMessagePath(conversationID string) string {
	return s.paths.RoomConversationMessagePath(conversationID)
}

// AppendRoomMessage 追加一条 Room 共享消息。
func (s *SessionFileStore) AppendRoomMessage(conversationID string, message sessiondomain.Message) error {
	return s.appendJSONL(s.paths.RoomConversationMessagePath(conversationID), message)
}

// ListSessions 读取某个 workspace 下的全部文件会话。
func (s *SessionFileStore) ListSessions(workspacePath string) ([]sessiondomain.Session, error) {
	sessionRoot := filepath.Join(workspacePath, ".agents", "sessions")
	entries, err := os.ReadDir(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []sessiondomain.Session{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]sessiondomain.Session, 0, len(entries))
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
func (s *SessionFileStore) FindSession(workspacePaths []string, sessionKey string) (*sessiondomain.Session, string, error) {
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
func (s *SessionFileStore) UpsertSession(workspacePath string, item sessiondomain.Session) (*sessiondomain.Session, error) {
	metaPath := s.paths.SessionMetaPath(workspacePath, item.SessionKey)
	messagePath := s.paths.SessionMessagePath(workspacePath, item.SessionKey)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return nil, err
	}

	// 中文注释：这里直接以 Go 模型作为 meta 真相源，避免再复制一套弱类型结构。
	payload, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(metaPath, payload, 0o644); err != nil {
		return nil, err
	}
	if _, err = os.Stat(messagePath); errors.Is(err, os.ErrNotExist) {
		if err = os.WriteFile(messagePath, []byte(""), 0o644); err != nil {
			return nil, err
		}
	} else if err != nil {
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

// AppendSessionMessage 追加一条完整消息到 messages.jsonl。
func (s *SessionFileStore) AppendSessionMessage(workspacePath string, sessionKey string, message sessiondomain.Message) error {
	return s.appendJSONL(s.paths.SessionMessagePath(workspacePath, sessionKey), message)
}

// AppendSessionCost 追加一条成本账本记录到 telemetry_cost.jsonl。
func (s *SessionFileStore) AppendSessionCost(workspacePath string, sessionKey string, row map[string]any) error {
	return s.appendJSONL(s.paths.SessionCostPath(workspacePath, sessionKey), row)
}

// ReadSessionMessages 读取 workspace 会话消息。
func (s *SessionFileStore) ReadSessionMessages(workspacePaths []string, sessionKey string) ([]sessiondomain.Message, error) {
	for _, workspacePath := range workspacePaths {
		rows, err := s.readMessagesFromPath(s.paths.SessionMessagePath(workspacePath, sessionKey))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return compactMessages(rows), nil
	}
	return []sessiondomain.Message{}, nil
}

// ReadRoomMessages 读取 Room 共享流历史。
func (s *SessionFileStore) ReadRoomMessages(logPath string) ([]sessiondomain.Message, error) {
	rows, err := s.readMessagesFromPath(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return []sessiondomain.Message{}, nil
	}
	if err != nil {
		return nil, err
	}
	return compactMessages(rows), nil
}

// ReadSessionCostSummary 读取或回算 Session 成本汇总。
func (s *SessionFileStore) ReadSessionCostSummary(workspacePaths []string, sessionKey string, fallbackAgentID string) (sessiondomain.CostSummary, error) {
	for _, workspacePath := range workspacePaths {
		summaryPath := s.paths.SessionCostSummaryPath(workspacePath, sessionKey)
		item, err := s.readCostSummaryFile(summaryPath)
		if errors.Is(err, os.ErrNotExist) {
			logRows, logErr := s.readJSONL(s.paths.SessionCostPath(workspacePath, sessionKey))
			if errors.Is(logErr, os.ErrNotExist) {
				continue
			}
			if logErr != nil {
				return sessiondomain.CostSummary{}, logErr
			}
			if len(logRows) == 0 {
				continue
			}
			return buildCostSummaryFromRows(sessionKey, fallbackAgentID, logRows), nil
		}
		if err != nil {
			return sessiondomain.CostSummary{}, err
		}
		if strings.TrimSpace(item.AgentID) == "" {
			item.AgentID = strings.TrimSpace(fallbackAgentID)
		}
		if strings.TrimSpace(item.SessionKey) == "" {
			item.SessionKey = sessionKey
		}
		return item, nil
	}
	return defaultCostSummary(sessionKey, fallbackAgentID), nil
}

func (s *SessionFileStore) readSessionMeta(metaPath string) (sessiondomain.Session, error) {
	payload, err := os.ReadFile(metaPath)
	if err != nil {
		return sessiondomain.Session{}, err
	}
	var item sessiondomain.Session
	if err = json.Unmarshal(payload, &item); err != nil {
		return sessiondomain.Session{}, err
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
	if item.RoomSessionID == nil {
		if value := stringFromAny(item.Options["room_session_id"]); value != "" {
			item.RoomSessionID = stringPointer(value)
		}
	}
	return item, nil
}

func (s *SessionFileStore) readMessagesFromPath(path string) ([]sessiondomain.Message, error) {
	rows, err := s.readJSONL(path)
	if err != nil {
		return nil, err
	}
	result := make([]sessiondomain.Message, 0, len(rows))
	for _, row := range rows {
		result = append(result, sessiondomain.Message(row))
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
		rows = append(rows, item)
	}
	return rows, reader.Err()
}

func (s *SessionFileStore) readCostSummaryFile(path string) (sessiondomain.CostSummary, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return sessiondomain.CostSummary{}, err
	}
	var item sessiondomain.CostSummary
	if err = json.Unmarshal(payload, &item); err != nil {
		return sessiondomain.CostSummary{}, err
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	return item, nil
}

func compactMessages(rows []sessiondomain.Message) []sessiondomain.Message {
	latestByID := make(map[string]sessiondomain.Message, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
		if messageID == "" {
			continue
		}
		if _, exists := latestByID[messageID]; !exists {
			order = append(order, messageID)
		}
		latestByID[messageID] = row
	}

	compacted := make([]sessiondomain.Message, 0, len(order))
	for _, messageID := range order {
		compacted = append(compacted, latestByID[messageID])
	}
	sort.Slice(compacted, func(i int, j int) bool {
		return messageTimestamp(compacted[i]) < messageTimestamp(compacted[j])
	})
	return compacted
}

func defaultCostSummary(sessionKey string, agentID string) sessiondomain.CostSummary {
	return sessiondomain.CostSummary{
		AgentID:    strings.TrimSpace(agentID),
		SessionKey: sessionKey,
		SessionID:  "",
		UpdatedAt:  time.Now().UTC(),
	}
}

func buildCostSummaryFromRows(sessionKey string, fallbackAgentID string, rows []map[string]any) sessiondomain.CostSummary {
	summary := defaultCostSummary(sessionKey, fallbackAgentID)
	if len(rows) == 0 {
		return summary
	}

	for _, row := range rows {
		summary.TotalInputTokens += intFromAny(row["input_tokens"])
		summary.TotalOutputTokens += intFromAny(row["output_tokens"])
		summary.TotalCacheCreationInputTokens += intFromAny(row["cache_creation_input_tokens"])
		summary.TotalCacheReadInputTokens += intFromAny(row["cache_read_input_tokens"])
		summary.TotalCostUSD += floatFromAny(row["total_cost_usd"])
		if strings.TrimSpace(stringFromAny(row["subtype"])) != "" && strings.TrimSpace(stringFromAny(row["subtype"])) != "success" {
			summary.ErrorRounds++
		}
	}

	latest := rows[len(rows)-1]
	summary.TotalTokens = summary.TotalInputTokens + summary.TotalOutputTokens
	summary.CompletedRounds = len(rows)
	summary.SessionID = stringFromAny(latest["session_id"])
	if summary.AgentID == "" {
		summary.AgentID = firstNonEmpty(
			stringFromAny(latest["agent_id"]),
			strings.TrimSpace(fallbackAgentID),
		)
	}
	if value := strings.TrimSpace(stringFromAny(latest["round_id"])); value != "" {
		summary.LastRoundID = stringPointer(value)
	}
	if value := intFromAny(latest["duration_ms"]); value > 0 {
		summary.LastRunDurationMS = intPointer(value)
	}
	if value := floatFromAny(latest["total_cost_usd"]); value > 0 {
		summary.LastRunCostUSD = floatPointer(value)
	}
	summary.UpdatedAt = time.Now().UTC()
	return summary
}

func messageTimestamp(row sessiondomain.Message) int64 {
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
