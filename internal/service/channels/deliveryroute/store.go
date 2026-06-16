package deliveryroute

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type Store struct {
	db                      *sql.DB
	isPostgres              bool
	idFactory               func(string) string
	rememberRouteQuery      string
	latestRouteQuery        string
	latestSessionRouteQuery string
}

type rememberedRoute struct {
	RouteID    string
	SessionKey string
	Target     channelcontract.DeliveryTarget
	Enabled    bool
}

func NewStore(cfg config.Config, db *sql.DB) *Store {
	store := &Store{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
		idFactory:  channelcontract.NewID,
	}
	store.rememberRouteQuery = fmt.Sprintf(`
INSERT INTO automation_delivery_routes (
    route_id,
    agent_id,
    session_key,
    mode,
    channel,
    "to",
    account_id,
    thread_id,
    enabled,
    created_at,
    updated_at
) VALUES (%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(route_id) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    session_key = EXCLUDED.session_key,
    mode = EXCLUDED.mode,
    channel = EXCLUDED.channel,
    "to" = EXCLUDED."to",
    account_id = EXCLUDED.account_id,
    thread_id = EXCLUDED.thread_id,
    enabled = EXCLUDED.enabled,
    updated_at = CURRENT_TIMESTAMP`,
		store.bindList(9),
	)
	store.latestRouteQuery = `
SELECT
    route_id,
    session_key,
    mode,
    channel,
    "to",
    account_id,
    thread_id,
    enabled
FROM automation_delivery_routes
WHERE agent_id = ` + store.bind(1) + `
  AND COALESCE(session_key, '') = ''
ORDER BY updated_at DESC, route_id DESC
LIMIT 1`
	store.latestSessionRouteQuery = `
SELECT
    route_id,
    session_key,
    mode,
    channel,
    "to",
    account_id,
    thread_id,
    enabled
FROM automation_delivery_routes
WHERE agent_id = ` + store.bind(1) + `
  AND session_key = ` + store.bind(2) + `
ORDER BY updated_at DESC, route_id DESC
LIMIT 1`
	return store
}

func (m *Store) bind(index int) string {
	if m.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (m *Store) bindList(count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		items = append(items, m.bind(index))
	}
	return strings.Join(items, ",")
}

// GetLastRoute 读取最近一次成功投递的显式目标。
func (m *Store) GetLastRoute(ctx context.Context, agentID string) (*channelcontract.DeliveryTarget, error) {
	row, err := m.getLatestRouteRow(ctx, agentID)
	return normalizedRememberedTarget(row, err)
}

// GetSessionRoute 读取指定 session 最近一次成功投递的显式目标。
func (m *Store) GetSessionRoute(ctx context.Context, agentID string, sessionKey string) (*channelcontract.DeliveryTarget, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return m.GetLastRoute(ctx, agentID)
	}
	row, err := m.getLatestSessionRouteRow(ctx, agentID, sessionKey)
	return normalizedRememberedTarget(row, err)
}

func normalizedRememberedTarget(row *rememberedRoute, err error) (*channelcontract.DeliveryTarget, error) {
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !row.Enabled || row.Target.Mode != channelcontract.DeliveryModeExplicit {
		return nil, nil
	}
	normalized := row.Target.Normalized()
	if normalized.Channel == "" || normalized.To == "" {
		return nil, nil
	}
	return &normalized, nil
}

// RememberRoute 刷新最近一次成功目标。
func (m *Store) RememberRoute(ctx context.Context, agentID string, target channelcontract.DeliveryTarget) (*channelcontract.DeliveryTarget, error) {
	return m.rememberRoute(ctx, agentID, "", target)
}

// RememberSessionRoute 刷新指定 session 最近一次成功目标。
func (m *Store) RememberSessionRoute(ctx context.Context, agentID string, sessionKey string, target channelcontract.DeliveryTarget) (*channelcontract.DeliveryTarget, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return m.RememberRoute(ctx, agentID, target)
	}
	return m.rememberRoute(ctx, agentID, sessionKey, target)
}

func (m *Store) rememberRoute(
	ctx context.Context,
	agentID string,
	sessionKey string,
	target channelcontract.DeliveryTarget,
) (*channelcontract.DeliveryTarget, error) {
	normalized := target.Normalized()
	if normalized.Mode == channelcontract.DeliveryModeNone || normalized.Mode == channelcontract.DeliveryModeLast {
		normalized.Mode = channelcontract.DeliveryModeExplicit
	}
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	routeID := m.idFactory("route")
	existing, err := m.getLatestRouteRowForScope(ctx, agentID, sessionKey)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil && strings.TrimSpace(existing.RouteID) != "" {
		routeID = existing.RouteID
	}

	_, err = m.db.ExecContext(
		ctx,
		m.rememberRouteQuery,
		routeID,
		strings.TrimSpace(agentID),
		strings.TrimSpace(sessionKey),
		channelcontract.DeliveryModeExplicit,
		channelcontract.NullableString(normalized.Channel),
		channelcontract.NullableString(normalized.To),
		channelcontract.NullableString(normalized.AccountID),
		channelcontract.NullableString(normalized.ThreadID),
		true,
	)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (m *Store) getLatestRouteRow(ctx context.Context, agentID string) (*rememberedRoute, error) {
	row := m.db.QueryRowContext(ctx, m.latestRouteQuery, strings.TrimSpace(agentID))
	return scanRememberedRoute(row)
}

func (m *Store) getLatestSessionRouteRow(ctx context.Context, agentID string, sessionKey string) (*rememberedRoute, error) {
	row := m.db.QueryRowContext(ctx, m.latestSessionRouteQuery, strings.TrimSpace(agentID), strings.TrimSpace(sessionKey))
	return scanRememberedRoute(row)
}

func (m *Store) getLatestRouteRowForScope(ctx context.Context, agentID string, sessionKey string) (*rememberedRoute, error) {
	if strings.TrimSpace(sessionKey) != "" {
		return m.getLatestSessionRouteRow(ctx, agentID, sessionKey)
	}
	return m.getLatestRouteRow(ctx, agentID)
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanRememberedRoute(row sqlScanner) (*rememberedRoute, error) {
	var (
		item      rememberedRoute
		channel   sql.NullString
		toValue   sql.NullString
		accountID sql.NullString
		threadID  sql.NullString
	)
	if err := row.Scan(
		&item.RouteID,
		&item.SessionKey,
		&item.Target.Mode,
		&channel,
		&toValue,
		&accountID,
		&threadID,
		&item.Enabled,
	); err != nil {
		return nil, err
	}
	item.SessionKey = strings.TrimSpace(item.SessionKey)
	item.Target.Channel = channelcontract.NullStringValue(channel)
	item.Target.To = channelcontract.NullStringValue(toValue)
	item.Target.AccountID = channelcontract.NullStringValue(accountID)
	item.Target.ThreadID = channelcontract.NullStringValue(threadID)
	item.Target = item.Target.Normalized()
	return &item, nil
}
