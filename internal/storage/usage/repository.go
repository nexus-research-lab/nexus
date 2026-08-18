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

const usageAttributionUpdateQueryTemplate = `UPDATE token_usage_records SET
    goal_scope = CASE WHEN goal_scope = 'unknown' AND %s <> 'unknown' THEN %s ELSE goal_scope END,
    execution_scope = CASE WHEN execution_scope = 'unknown' AND %s <> 'unknown' THEN %s ELSE execution_scope END,
    responsibility_lane = CASE WHEN responsibility_lane = 'unknown' AND %s <> 'unknown' THEN %s ELSE responsibility_lane END,
    runtime_kind = CASE WHEN runtime_kind = 'unknown' AND %s <> 'unknown' THEN %s ELSE runtime_kind END,
    provider_fingerprint = CASE WHEN provider_fingerprint = '' AND %s <> '' THEN %s ELSE provider_fingerprint END,
    model_fingerprint = CASE WHEN model_fingerprint = '' AND %s <> '' THEN %s ELSE model_fingerprint END,
	host_tool_surface_complete = host_tool_surface_complete OR %s,
    tool_policy_fingerprint = CASE WHEN tool_policy_fingerprint = '' AND %s <> '' THEN %s ELSE tool_policy_fingerprint END,
    mcp_servers_fingerprint = CASE WHEN mcp_servers_fingerprint = '' AND %s <> '' THEN %s ELSE mcp_servers_fingerprint END,
    tool_surface_fingerprint = CASE WHEN tool_surface_fingerprint = '' AND %s <> '' THEN %s ELSE tool_surface_fingerprint END
WHERE owner_user_id = %s AND usage_key = %s`

const cacheSegmentsQueryTemplate = `SELECT
    source,
    goal_scope,
    execution_scope,
    responsibility_lane,
    runtime_kind,
    provider_fingerprint,
    model_fingerprint,
	host_tool_surface_complete,
    tool_policy_fingerprint,
    mcp_servers_fingerprint,
    tool_surface_fingerprint,
    COALESCE(SUM(input_tokens), 0),
    COALESCE(SUM(output_tokens), 0),
    COALESCE(SUM(cache_creation_input_tokens), 0),
    COALESCE(SUM(cache_read_input_tokens), 0),
    COALESCE(SUM(total_tokens), 0),
    COUNT(*)
FROM token_usage_records
WHERE owner_user_id = %s
GROUP BY source, goal_scope, execution_scope, responsibility_lane, runtime_kind,
	provider_fingerprint, model_fingerprint,
    host_tool_surface_complete, tool_policy_fingerprint, mcp_servers_fingerprint,
    tool_surface_fingerprint
ORDER BY source, goal_scope, execution_scope, responsibility_lane, runtime_kind,
    provider_fingerprint, model_fingerprint, tool_surface_fingerprint`

// Record 表示 token usage ledger 的一条持久化记录。
// Repository 封装 token usage ledger 的 SQL 读写。
type Repository struct {
	db                     *sql.DB
	isPostgres             bool
	upsertQuery            string
	summaryQuery           string
	attributionUpdateQuery string
	cacheSegmentsQuery     string
}

// NewRepository 创建 token usage SQL 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	repository := &Repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
	repository.upsertQuery = repository.buildUpsertQuery()
	repository.summaryQuery = repository.buildSummaryQuery()
	repository.attributionUpdateQuery = repository.buildAttributionUpdateQuery()
	repository.cacheSegmentsQuery = repository.buildCacheSegmentsQuery()
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

// UpdateCacheAttribution best-effort 更新 cache 归因；调用方应把它与主 usage
// settlement 隔离，避免升级窗口或观测字段故障反向影响计费账本。
func (r *Repository) UpdateCacheAttribution(ctx context.Context, item Record) error {
	_, err := r.db.ExecContext(
		ctx,
		r.attributionUpdateQuery,
		item.GoalScope, item.GoalScope,
		item.ExecutionScope, item.ExecutionScope,
		item.ResponsibilityLane, item.ResponsibilityLane,
		item.RuntimeKind, item.RuntimeKind,
		item.ProviderFingerprint, item.ProviderFingerprint,
		item.ModelFingerprint, item.ModelFingerprint,
		item.HostToolSurfaceComplete,
		item.ToolPolicyFingerprint, item.ToolPolicyFingerprint,
		item.MCPServersFingerprint, item.MCPServersFingerprint,
		item.ToolSurfaceFingerprint, item.ToolSurfaceFingerprint,
		item.OwnerUserID,
		item.UsageKey,
	)
	return err
}

func (r *Repository) buildAttributionUpdateQuery() string {
	bindings := make([]any, 21)
	for index := range bindings {
		bindings[index] = r.bind(index + 1)
	}
	return fmt.Sprintf(usageAttributionUpdateQueryTemplate, bindings...)
}

// CacheSegments 返回按低基数宿主 cache surface 分段的 provider usage。
func (r *Repository) CacheSegments(ctx context.Context, ownerUserID string) ([]CacheSegment, error) {
	rows, err := r.db.QueryContext(ctx, r.cacheSegmentsQuery, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	segments := make([]CacheSegment, 0)
	for rows.Next() {
		var segment CacheSegment
		var messageCount int64
		if err := rows.Scan(
			&segment.Source,
			&segment.GoalScope,
			&segment.ExecutionScope,
			&segment.ResponsibilityLane,
			&segment.RuntimeKind,
			&segment.ProviderFingerprint,
			&segment.ModelFingerprint,
			&segment.HostToolSurfaceComplete,
			&segment.ToolPolicyFingerprint,
			&segment.MCPServersFingerprint,
			&segment.ToolSurfaceFingerprint,
			&segment.InputTokens,
			&segment.OutputTokens,
			&segment.CacheCreationInputTokens,
			&segment.CacheReadInputTokens,
			&segment.TotalTokens,
			&messageCount,
		); err != nil {
			return nil, err
		}
		segment.MessageCount = int(messageCount)
		segments = append(segments, segment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return segments, nil
}

func (r *Repository) buildCacheSegmentsQuery() string {
	return fmt.Sprintf(cacheSegmentsQueryTemplate, r.bind(1))
}
