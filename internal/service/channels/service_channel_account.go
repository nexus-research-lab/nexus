package channels

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type channelAccountRow struct {
	OwnerUserID          string
	ChannelType          string
	AccountID            string
	UserID               string
	Status               string
	ConfigJSON           string
	CredentialsEncrypted sql.NullString
	LastError            sql.NullString
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (s *ControlService) listChannelAccountRows(
	ctx context.Context,
	ownerUserID string,
	channelType string,
) ([]channelAccountRow, error) {
	query := `
SELECT owner_user_id, channel_type, account_id, user_id, status, config_json,
       credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_accounts
WHERE owner_user_id = ` + s.bind(1) + `
  AND channel_type = ` + s.bind(2) + `
ORDER BY updated_at DESC, account_id DESC`
	rows, err := s.db.QueryContext(
		ctx,
		query,
		normalizeChannelOwnerUserID(ownerUserID),
		normalizeIMChannelType(channelType),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []channelAccountRow{}
	for rows.Next() {
		item, scanErr := scanChannelAccountScanner(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func (s *ControlService) channelAccountsByType(ctx context.Context, ownerUserID string) (map[string][]channelAccountRow, error) {
	query := `
SELECT owner_user_id, channel_type, account_id, user_id, status, config_json,
       credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_accounts
WHERE owner_user_id = ` + s.bind(1) + `
ORDER BY channel_type ASC, updated_at DESC, account_id DESC`
	rows, err := s.db.QueryContext(ctx, query, normalizeChannelOwnerUserID(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]channelAccountRow{}
	for rows.Next() {
		item, scanErr := scanChannelAccountScanner(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[item.ChannelType] = append(result[item.ChannelType], *item)
	}
	return result, rows.Err()
}

func (s *ControlService) upsertChannelAccountRow(ctx context.Context, row channelAccountRow) error {
	if s.driver == "pgx" {
		query := `
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, user_id, status, config_json,
    credentials_encrypted, last_error
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (owner_user_id, channel_type, account_id) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    status = EXCLUDED.status,
    config_json = EXCLUDED.config_json,
    credentials_encrypted = EXCLUDED.credentials_encrypted,
    last_error = EXCLUDED.last_error,
    updated_at = CURRENT_TIMESTAMP`
		_, err := s.db.ExecContext(
			ctx,
			query,
			strings.TrimSpace(row.OwnerUserID),
			normalizeIMChannelType(row.ChannelType),
			strings.TrimSpace(row.AccountID),
			strings.TrimSpace(row.UserID),
			firstNonEmpty(normalizeChannelConfigStatus(row.Status), ChannelConfigStatusConnected),
			firstNonEmpty(row.ConfigJSON, "{}"),
			nullableString(row.CredentialsEncrypted.String),
			nullableString(row.LastError.String),
		)
		return err
	}
	query := `
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, user_id, status, config_json,
    credentials_encrypted, last_error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(owner_user_id, channel_type, account_id) DO UPDATE SET
    user_id = excluded.user_id,
    status = excluded.status,
    config_json = excluded.config_json,
    credentials_encrypted = excluded.credentials_encrypted,
    last_error = excluded.last_error,
    updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(
		ctx,
		query,
		strings.TrimSpace(row.OwnerUserID),
		normalizeIMChannelType(row.ChannelType),
		strings.TrimSpace(row.AccountID),
		strings.TrimSpace(row.UserID),
		firstNonEmpty(normalizeChannelConfigStatus(row.Status), ChannelConfigStatusConnected),
		firstNonEmpty(row.ConfigJSON, "{}"),
		nullableString(row.CredentialsEncrypted.String),
		nullableString(row.LastError.String),
	)
	return err
}

func (s *ControlService) deleteChannelAccountRows(ctx context.Context, ownerUserID string, channelType string) error {
	query := "DELETE FROM im_channel_accounts WHERE owner_user_id = " + s.bind(1) + " AND channel_type = " + s.bind(2)
	_, err := s.db.ExecContext(ctx, query, normalizeChannelOwnerUserID(ownerUserID), normalizeIMChannelType(channelType))
	return err
}

func (s *ControlService) deleteChannelAccountRow(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	accountID string,
) (bool, error) {
	query := "DELETE FROM im_channel_accounts WHERE owner_user_id = " + s.bind(1) + " AND channel_type = " + s.bind(2) + " AND account_id = " + s.bind(3)
	result, err := s.db.ExecContext(
		ctx,
		query,
		normalizeChannelOwnerUserID(ownerUserID),
		normalizeIMChannelType(channelType),
		strings.TrimSpace(accountID),
	)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	return affected > 0, nil
}

func scanChannelAccountScanner(row sqlScanner) (*channelAccountRow, error) {
	var item channelAccountRow
	err := row.Scan(
		&item.OwnerUserID,
		&item.ChannelType,
		&item.AccountID,
		&item.UserID,
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
	item.OwnerUserID = normalizeChannelOwnerUserID(item.OwnerUserID)
	item.ChannelType = normalizeIMChannelType(item.ChannelType)
	item.AccountID = strings.TrimSpace(item.AccountID)
	item.UserID = strings.TrimSpace(item.UserID)
	item.Status = normalizeChannelConfigStatus(item.Status)
	return &item, nil
}

func channelAccountViews(rows []channelAccountRow) []ChannelAccountView {
	result := make([]ChannelAccountView, 0, len(rows))
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled {
			continue
		}
		result = append(result, ChannelAccountView{
			AccountID: row.AccountID,
			UserID:    row.UserID,
			Status:    firstNonEmpty(row.Status, ChannelConfigStatusConnected),
			LastError: nullStringValue(row.LastError),
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return result
}

func hasConnectedChannelAccount(rows []channelAccountRow) bool {
	for _, row := range rows {
		if row.Status == ChannelConfigStatusConnected {
			return true
		}
	}
	return false
}
