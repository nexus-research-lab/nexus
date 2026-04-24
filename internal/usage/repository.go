package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type record struct {
	OwnerUserID              string
	UsageKey                 string
	Source                   string
	SessionKey               string
	MessageID                string
	RoundID                  string
	AgentID                  string
	RoomID                   string
	ConversationID           string
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	TotalTokens              int64
	OccurredAt               time.Time
}

type repository struct {
	db         *sql.DB
	isPostgres bool
}

func newRepository(cfg config.Config, db *sql.DB) *repository {
	return &repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

func (r *repository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (r *repository) bindList(count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		items = append(items, r.bind(index))
	}
	return strings.Join(items, ", ")
}

func (r *repository) upsert(ctx context.Context, item record) error {
	now := time.Now().UTC()
	query := fmt.Sprintf(`INSERT INTO token_usage_records (
    owner_user_id,
    usage_key,
    source,
    session_key,
    message_id,
    round_id,
    agent_id,
    room_id,
    conversation_id,
    input_tokens,
    output_tokens,
    cache_creation_input_tokens,
    cache_read_input_tokens,
    total_tokens,
    occurred_at,
    created_at,
    updated_at
) VALUES (%s)
ON CONFLICT(owner_user_id, usage_key) DO UPDATE SET
    source = excluded.source,
    session_key = excluded.session_key,
    message_id = excluded.message_id,
    round_id = excluded.round_id,
    agent_id = excluded.agent_id,
    room_id = excluded.room_id,
    conversation_id = excluded.conversation_id,
    input_tokens = excluded.input_tokens,
    output_tokens = excluded.output_tokens,
    cache_creation_input_tokens = excluded.cache_creation_input_tokens,
    cache_read_input_tokens = excluded.cache_read_input_tokens,
    total_tokens = excluded.total_tokens,
    occurred_at = excluded.occurred_at,
    updated_at = excluded.updated_at`, r.bindList(17))

	_, err := r.db.ExecContext(
		ctx,
		query,
		item.OwnerUserID,
		item.UsageKey,
		item.Source,
		item.SessionKey,
		item.MessageID,
		item.RoundID,
		item.AgentID,
		item.RoomID,
		item.ConversationID,
		item.InputTokens,
		item.OutputTokens,
		item.CacheCreationInputTokens,
		item.CacheReadInputTokens,
		item.TotalTokens,
		item.OccurredAt,
		now,
		now,
	)
	return err
}

func (r *repository) summary(ctx context.Context, ownerUserID string, now time.Time) (Summary, error) {
	row := r.db.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT
    COALESCE(SUM(input_tokens), 0),
    COALESCE(SUM(output_tokens), 0),
    COALESCE(SUM(cache_creation_input_tokens), 0),
    COALESCE(SUM(cache_read_input_tokens), 0),
    COALESCE(SUM(total_tokens), 0),
    COUNT(DISTINCT session_key),
    COUNT(*)
FROM token_usage_records
WHERE owner_user_id = %s`, r.bind(1)),
		ownerUserID,
	)
	var result Summary
	var (
		sessionCount int64
		messageCount int64
	)
	if err := row.Scan(
		&result.InputTokens,
		&result.OutputTokens,
		&result.CacheCreationInputTokens,
		&result.CacheReadInputTokens,
		&result.TotalTokens,
		&sessionCount,
		&messageCount,
	); err != nil {
		return Summary{}, err
	}
	result.SessionCount = int(sessionCount)
	result.MessageCount = int(messageCount)
	result.UpdatedAt = now.UTC().Format(time.RFC3339)
	return result, nil
}
