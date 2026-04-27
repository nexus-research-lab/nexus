package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

// Service 负责用户级 token usage ledger。
type Service struct {
	repository *repository
	now        func() time.Time
}

// NewServiceWithDB 使用共享 DB 创建 usage 服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository: newRepository(cfg, db),
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// RecordMessageUsage 把单条结果消息里的 usage 写入持久 ledger。
func (s *Service) RecordMessageUsage(ctx context.Context, input RecordInput) error {
	record, ok := s.buildRecord(input)
	if !ok {
		return nil
	}
	return s.repository.upsert(ctx, record)
}

// Summary 返回用户级 token 用量汇总。
func (s *Service) Summary(ctx context.Context, ownerUserID string) (Summary, error) {
	ownerUserID = normalizeOwnerUserID(ownerUserID)
	return s.repository.summary(ctx, ownerUserID, s.now())
}

func (s *Service) buildRecord(input RecordInput) (record, bool) {
	ownerUserID := normalizeOwnerUserID(input.OwnerUserID)
	sessionKey := strings.TrimSpace(input.SessionKey)
	messageID := strings.TrimSpace(input.MessageID)
	roundID := strings.TrimSpace(input.RoundID)
	if sessionKey == "" || (messageID == "" && roundID == "") {
		return record{}, false
	}

	inputTokens := int64FromAny(input.Usage["input_tokens"])
	outputTokens := int64FromAny(input.Usage["output_tokens"])
	cacheCreationTokens := int64FromAny(input.Usage["cache_creation_input_tokens"])
	cacheReadTokens := int64FromAny(input.Usage["cache_read_input_tokens"])
	totalTokens := int64FromAny(input.Usage["total_tokens"])
	if totalTokens <= 0 {
		totalTokens = inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens
	}
	if totalTokens <= 0 {
		return record{}, false
	}

	occurredAt := input.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = s.now()
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "runtime"
	}
	usageKey := buildUsageKey(sessionKey, messageID, roundID)
	return record{
		OwnerUserID:              ownerUserID,
		UsageKey:                 usageKey,
		Source:                   source,
		SessionKey:               sessionKey,
		MessageID:                messageID,
		RoundID:                  roundID,
		AgentID:                  strings.TrimSpace(input.AgentID),
		RoomID:                   strings.TrimSpace(input.RoomID),
		ConversationID:           strings.TrimSpace(input.ConversationID),
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		CacheCreationInputTokens: cacheCreationTokens,
		CacheReadInputTokens:     cacheReadTokens,
		TotalTokens:              totalTokens,
		OccurredAt:               occurredAt,
	}, true
}

func normalizeOwnerUserID(ownerUserID string) string {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return authsvc.SystemUserID
	}
	return ownerUserID
}

func buildUsageKey(sessionKey string, messageID string, roundID string) string {
	if messageID != "" {
		return sessionKey + ":" + messageID
	}
	return sessionKey + ":" + roundID
}

func int64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0
		}
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(typed.String(), 64); err == nil {
			return int64(parsed)
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

// OwnerUserIDFromContext 暴露给运行链路捕获当前用户作用域。
func OwnerUserIDFromContext(ctx context.Context) string {
	if userID, ok := authsvc.CurrentUserID(ctx); ok {
		return userID
	}
	return authsvc.SystemUserID
}

func timestampFromAny(value any) time.Time {
	switch typed := value.(type) {
	case int64:
		return time.UnixMilli(typed).UTC()
	case int:
		return time.UnixMilli(int64(typed)).UTC()
	case float64:
		return time.UnixMilli(int64(typed)).UTC()
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return time.UnixMilli(parsed).UTC()
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return time.UnixMilli(parsed).UTC()
		}
	}
	return time.Time{}
}

// MessageRecordInput 从消息 map 构造可写 ledger 的输入。
func MessageRecordInput(ownerUserID string, source string, message map[string]any) RecordInput {
	usage, _ := message["usage"].(map[string]any)
	return RecordInput{
		OwnerUserID:    ownerUserID,
		Source:         source,
		SessionKey:     stringValue(message["session_key"]),
		MessageID:      stringValue(message["message_id"]),
		RoundID:        stringValue(message["round_id"]),
		AgentID:        stringValue(message["agent_id"]),
		RoomID:         stringValue(message["room_id"]),
		ConversationID: stringValue(message["conversation_id"]),
		Usage:          usage,
		OccurredAt:     timestampFromAny(message["timestamp"]),
	}
}

func stringValue(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}
