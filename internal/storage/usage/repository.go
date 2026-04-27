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

const usageUpsertQueryTemplate = `INSERT INTO token_usage_records (
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
    updated_at = excluded.updated_at`

const usageSummaryQueryTemplate = `SELECT
    COALESCE(SUM(input_tokens), 0),
    COALESCE(SUM(output_tokens), 0),
    COALESCE(SUM(cache_creation_input_tokens), 0),
    COALESCE(SUM(cache_read_input_tokens), 0),
    COALESCE(SUM(total_tokens), 0),
    COUNT(DISTINCT session_key),
    COUNT(*)
FROM token_usage_records
WHERE owner_user_id = %s`

// Record 表示 token usage ledger 的一条持久化记录。
// Repository 封装 token usage ledger 的 SQL 读写。
type Repository struct {
	db           *sql.DB
	isPostgres   bool
	upsertQuery  string
	summaryQuery string
}

// NewRepository 创建 token usage SQL 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	repository := &Repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
	repository.upsertQuery = repository.buildUpsertQuery()
	repository.summaryQuery = repository.buildSummaryQuery()
	return repository
}

func (r *Repository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (r *Repository) bindList(count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		items = append(items, r.bind(index))
	}
	return strings.Join(items, ", ")
}

func (r *Repository) Upsert(ctx context.Context, item Record) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(
		ctx,
		r.upsertQuery,
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

func (r *Repository) buildUpsertQuery() string {
	return fmt.Sprintf(usageUpsertQueryTemplate, r.bindList(17))
}

func (r *Repository) Summary(ctx context.Context, ownerUserID string, now time.Time) (Summary, error) {
	row := r.db.QueryRowContext(
		ctx,
		r.summaryQuery,
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

func (r *Repository) buildSummaryQuery() string {
	return fmt.Sprintf(usageSummaryQueryTemplate, r.bind(1))
}
