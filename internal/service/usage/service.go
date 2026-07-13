package usage

import (
	"cmp"
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	usagestore "github.com/nexus-research-lab/nexus/internal/storage/usage"
)

// Service 负责用户级 token usage ledger。
type Service struct {
	repository *usagestore.Repository
	now        func() time.Time
}

// NewServiceWithDB 使用共享 DB 创建 usage 服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository: usagestore.NewRepository(cfg, db),
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// RecordMessageUsage 把单条结果消息里的 usage 写入持久 ledger。
func (s *Service) RecordMessageUsage(ctx context.Context, input RecordInput) error {
	record, ok := s.buildRecord(input)
	if !ok {
		return nil
	}
	return s.repository.Upsert(ctx, record)
}

// Summary 返回用户级 token 用量汇总。
func (s *Service) Summary(ctx context.Context, ownerUserID string) (Summary, error) {
	ownerUserID = normalizeOwnerUserID(ownerUserID)
	stored, err := s.repository.Summary(ctx, ownerUserID, s.now())
	if err != nil {
		return Summary{}, err
	}
	return Summary{
		InputTokens:              stored.InputTokens,
		OutputTokens:             stored.OutputTokens,
		CacheCreationInputTokens: stored.CacheCreationInputTokens,
		CacheReadInputTokens:     stored.CacheReadInputTokens,
		TotalTokens:              stored.TotalTokens,
		SessionCount:             stored.SessionCount,
		MessageCount:             stored.MessageCount,
		UpdatedAt:                stored.UpdatedAt,
	}, nil
}

func (s *Service) buildRecord(input RecordInput) (usagestore.Record, bool) {
	ownerUserID := normalizeOwnerUserID(input.OwnerUserID)
	sessionKey := strings.TrimSpace(input.SessionKey)
	messageID := strings.TrimSpace(input.MessageID)
	roundID := strings.TrimSpace(input.RoundID)
	if sessionKey == "" || (messageID == "" && roundID == "") {
		return usagestore.Record{}, false
	}

	inputTokens := protocol.Int64FromAny(input.Usage["input_tokens"])
	outputTokens := protocol.Int64FromAny(input.Usage["output_tokens"])
	cacheCreationTokens := protocol.Int64FromAny(input.Usage["cache_creation_input_tokens"])
	cacheReadTokens := protocol.Int64FromAny(input.Usage["cache_read_input_tokens"])
	totalTokens := firstPositiveInt64(
		protocol.Int64FromAny(input.Usage["total_tokens"]),
		inputTokens+outputTokens+cacheCreationTokens+cacheReadTokens,
	)
	if totalTokens <= 0 {
		return usagestore.Record{}, false
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
	return usagestore.Record{
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

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizeOwnerUserID(ownerUserID string) string {
	return cmp.Or(strings.TrimSpace(ownerUserID), authctx.SystemUserID)
}

func buildUsageKey(sessionKey string, messageID string, roundID string) string {
	if messageID != "" {
		return sessionKey + ":" + messageID
	}
	return sessionKey + ":" + roundID
}

func timestampFromAny(value any) time.Time {
	timestamp := protocol.Int64FromAny(value)
	if timestamp == 0 {
		return time.Time{}
	}
	return time.UnixMilli(timestamp).UTC()
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

// MessageHasUsage 判断消息是否携带可入账的 token usage。
func MessageHasUsage(message map[string]any) bool {
	usage, _ := message["usage"].(map[string]any)
	if len(usage) == 0 {
		return false
	}
	return protocol.Int64FromAny(usage["input_tokens"]) > 0 ||
		protocol.Int64FromAny(usage["output_tokens"]) > 0 ||
		protocol.Int64FromAny(usage["cache_creation_input_tokens"]) > 0 ||
		protocol.Int64FromAny(usage["cache_read_input_tokens"]) > 0 ||
		protocol.Int64FromAny(usage["total_tokens"]) > 0
}

func stringValue(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}
