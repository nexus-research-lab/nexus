package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *ControlService) listChannelConfigRows(ctx context.Context, ownerUserID string) ([]channelConfigRow, error) {
	query := `
SELECT owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs
WHERE owner_user_id = ` + s.bind(1)
	rows, err := s.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannelConfigRows(rows)
}

func (s *ControlService) listAllChannelConfigRows(ctx context.Context) ([]channelConfigRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannelConfigRows(rows)
}

func (s *ControlService) getChannelConfigRow(ctx context.Context, ownerUserID string, channelType string) (*channelConfigRow, error) {
	query := `
SELECT owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs
WHERE owner_user_id = ` + s.bind(1) + " AND channel_type = " + s.bind(2)
	row := s.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(channelType))
	item, err := scanChannelConfigScanner(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s *ControlService) upsertChannelConfigRow(ctx context.Context, row channelConfigRow) error {
	if s.driver == "pgx" {
		query := `
INSERT INTO im_channel_configs (
    owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error
) VALUES ($1, $2, $3, $4, $5, $6, NULL)
ON CONFLICT (owner_user_id, channel_type) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    status = EXCLUDED.status,
    config_json = EXCLUDED.config_json,
    credentials_encrypted = EXCLUDED.credentials_encrypted,
    last_error = NULL,
    updated_at = CURRENT_TIMESTAMP`
		_, err := s.db.ExecContext(ctx, query, row.OwnerUserID, row.ChannelType, row.AgentID, row.Status, row.ConfigJSON, nullableString(row.CredentialsEncrypted.String))
		return err
	}
	query := `
INSERT INTO im_channel_configs (
    owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error
) VALUES (?, ?, ?, ?, ?, ?, NULL)
ON CONFLICT(owner_user_id, channel_type) DO UPDATE SET
    agent_id = excluded.agent_id,
    status = excluded.status,
    config_json = excluded.config_json,
    credentials_encrypted = excluded.credentials_encrypted,
    last_error = NULL,
    updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(ctx, query, row.OwnerUserID, row.ChannelType, row.AgentID, row.Status, row.ConfigJSON, nullableString(row.CredentialsEncrypted.String))
	return err
}

func (s *ControlService) updateChannelConfigRuntimeState(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	status string,
	lastError string,
) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	status = firstNonEmpty(normalizeChannelConfigStatus(status), ChannelConfigStatusConfigured)
	if s.driver == "pgx" {
		query := `
UPDATE im_channel_configs
SET status = $3, last_error = $4, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = $1 AND channel_type = $2`
		_, err := s.db.ExecContext(ctx, query, ownerUserID, channelType, status, nullableString(lastError))
		return err
	}
	query := `
UPDATE im_channel_configs
SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = ? AND channel_type = ?`
	_, err := s.db.ExecContext(ctx, query, status, nullableString(lastError), ownerUserID, channelType)
	return err
}

func scanChannelConfigRows(rows *sql.Rows) ([]channelConfigRow, error) {
	result := []channelConfigRow{}
	for rows.Next() {
		item, err := scanChannelConfigScanner(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func scanChannelConfigScanner(row sqlScanner) (*channelConfigRow, error) {
	var item channelConfigRow
	err := row.Scan(
		&item.OwnerUserID,
		&item.ChannelType,
		&item.AgentID,
		&item.Status,
		&item.ConfigJSON,
		&item.CredentialsEncrypted,
		&item.LastError,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.ChannelType = normalizeIMChannelType(item.ChannelType)
	return &item, nil
}
