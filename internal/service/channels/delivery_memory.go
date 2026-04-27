package channels

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type deliveryMemory struct {
	db         *sql.DB
	isPostgres bool
	idFactory  func(string) string
}

type rememberedRoute struct {
	RouteID string
	Target  DeliveryTarget
	Enabled bool
}

func newDeliveryMemory(cfg config.Config, db *sql.DB) *deliveryMemory {
	return &deliveryMemory{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
		idFactory:  newDeliveryID,
	}
}

func (m *deliveryMemory) bind(index int) string {
	if m.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

// GetLastRoute 读取最近一次成功投递的显式目标。
func (m *deliveryMemory) GetLastRoute(ctx context.Context, agentID string) (*DeliveryTarget, error) {
	row, err := m.getLatestRouteRow(ctx, agentID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !row.Enabled || row.Target.Mode != DeliveryModeExplicit {
		return nil, nil
	}
	normalized := row.Target.Normalized()
	if normalized.Channel == "" || normalized.To == "" {
		return nil, nil
	}
	return &normalized, nil
}

// RememberRoute 刷新最近一次成功目标。
func (m *deliveryMemory) RememberRoute(ctx context.Context, agentID string, target DeliveryTarget) (*DeliveryTarget, error) {
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone || normalized.Mode == DeliveryModeLast {
		normalized.Mode = DeliveryModeExplicit
	}
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	routeID := m.idFactory("route")
	existing, err := m.getLatestRouteRow(ctx, agentID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil && strings.TrimSpace(existing.RouteID) != "" {
		routeID = existing.RouteID
	}

	query := fmt.Sprintf(`
INSERT INTO automation_delivery_routes (
    route_id,
    agent_id,
    mode,
    channel,
    "to",
    account_id,
    thread_id,
    enabled,
    created_at,
    updated_at
) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(route_id) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    mode = EXCLUDED.mode,
    channel = EXCLUDED.channel,
    "to" = EXCLUDED."to",
    account_id = EXCLUDED.account_id,
    thread_id = EXCLUDED.thread_id,
    enabled = EXCLUDED.enabled,
    updated_at = CURRENT_TIMESTAMP`,
		m.bind(1), m.bind(2), m.bind(3), m.bind(4), m.bind(5), m.bind(6), m.bind(7), m.bind(8),
	)
	_, err = m.db.ExecContext(
		ctx,
		query,
		routeID,
		strings.TrimSpace(agentID),
		DeliveryModeExplicit,
		nullableString(normalized.Channel),
		nullableString(normalized.To),
		nullableString(normalized.AccountID),
		nullableString(normalized.ThreadID),
		true,
	)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (m *deliveryMemory) getLatestRouteRow(ctx context.Context, agentID string) (*rememberedRoute, error) {
	query := `
SELECT
    route_id,
    mode,
    channel,
    "to",
    account_id,
    thread_id,
    enabled
FROM automation_delivery_routes
WHERE agent_id = ` + m.bind(1) + `
ORDER BY updated_at DESC, route_id DESC
LIMIT 1`
	row := m.db.QueryRowContext(ctx, strings.TrimSpace(query), strings.TrimSpace(agentID))
	var (
		item      rememberedRoute
		channel   sql.NullString
		toValue   sql.NullString
		accountID sql.NullString
		threadID  sql.NullString
	)
	if err := row.Scan(
		&item.RouteID,
		&item.Target.Mode,
		&channel,
		&toValue,
		&accountID,
		&threadID,
		&item.Enabled,
	); err != nil {
		return nil, err
	}
	item.Target.Channel = nullStringValue(channel)
	item.Target.To = nullStringValue(toValue)
	item.Target.AccountID = nullStringValue(accountID)
	item.Target.ThreadID = nullStringValue(threadID)
	item.Target = item.Target.Normalized()
	return &item, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func newDeliveryID(prefix string) string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}
