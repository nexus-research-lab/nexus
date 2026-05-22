package goal

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func goalSelectQuery(where string) string {
	return `SELECT
    goal_id,
    session_key,
    objective,
    status,
    token_budget,
    token_used_input,
    token_used_output,
    token_used_cache_creation,
    token_used_cache_read,
    token_used_reasoning,
    token_used_total,
    continuation_count,
    empty_progress_count,
    version,
    created_by,
    created_at,
    updated_at,
    completed_at,
    blocked_at,
    cleared_at,
    last_error,
    metadata_json
FROM session_goals
WHERE ` + where
}

func scanGoal(scanner interface{ Scan(...any) error }) (protocol.Goal, error) {
	var (
		item         protocol.Goal
		status       string
		tokenBudget  sql.NullInt64
		createdBy    sql.NullString
		completedAt  sql.NullTime
		blockedAt    sql.NullTime
		clearedAt    sql.NullTime
		lastError    sql.NullString
		metadataJSON string
	)
	err := scanner.Scan(
		&item.ID,
		&item.SessionKey,
		&item.Objective,
		&status,
		&tokenBudget,
		&item.Usage.InputTokens,
		&item.Usage.OutputTokens,
		&item.Usage.CacheCreationInputTokens,
		&item.Usage.CacheReadInputTokens,
		&item.Usage.ReasoningTokens,
		&item.Usage.TotalTokens,
		&item.ContinuationCount,
		&item.EmptyProgressCount,
		&item.Version,
		&createdBy,
		&item.CreatedAt,
		&item.UpdatedAt,
		&completedAt,
		&blockedAt,
		&clearedAt,
		&lastError,
		&metadataJSON,
	)
	if err != nil {
		return protocol.Goal{}, err
	}
	item.Status = protocol.NormalizeGoalStatus(protocol.GoalStatus(status))
	item.TokenBudget = nullInt64ToPointer(tokenBudget)
	item.CreatedBy = nullStringValue(createdBy)
	item.CompletedAt = nullTimePointer(completedAt)
	item.BlockedAt = nullTimePointer(blockedAt)
	item.ClearedAt = nullTimePointer(clearedAt)
	item.LastError = nullStringValue(lastError)
	item.Metadata = parseMap(metadataJSON)
	return item, nil
}

func scanGoalEvent(scanner interface{ Scan(...any) error }) (protocol.GoalEvent, error) {
	var (
		item        protocol.GoalEvent
		source      string
		roundID     sql.NullString
		payloadJSON string
	)
	err := scanner.Scan(
		&item.ID,
		&item.GoalID,
		&item.SessionKey,
		&item.EventType,
		&source,
		&roundID,
		&payloadJSON,
		&item.CreatedAt,
	)
	if err != nil {
		return protocol.GoalEvent{}, err
	}
	item.Source = protocol.GoalUpdateSource(source)
	item.RoundID = nullStringValue(roundID)
	item.Payload = parseMap(payloadJSON)
	return item, nil
}

func nullInt64ToPointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func parseMap(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}
