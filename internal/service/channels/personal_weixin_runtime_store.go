// INPUT: 个人微信 owner/account 身份、iLink 不透明轮询游标与明确登录失效事实。
// OUTPUT: 账号行内的持久游标和需要重新扫码的 error 状态。
// POS: Channels 控制面实现 adapters.PersonalWeixinRuntimeStore 的 SQL 边界。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *ControlService) LoadPersonalWeixinCursor(
	ctx context.Context,
	ownerUserID string,
	accountID string,
) (string, error) {
	query := `
SELECT sync_cursor
FROM im_channel_accounts
WHERE owner_user_id = ` + s.bind(1) + `
  AND channel_type = ` + s.bind(2) + `
  AND account_id = ` + s.bind(3)
	var cursor string
	err := s.db.QueryRowContext(
		ctx,
		query,
		normalizeChannelOwnerUserID(ownerUserID),
		ChannelTypeWeixinPersonal,
		strings.TrimSpace(accountID),
	).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(cursor), err
}

func (s *ControlService) SavePersonalWeixinCursor(
	ctx context.Context,
	ownerUserID string,
	accountID string,
	cursor string,
) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	accountID = strings.TrimSpace(accountID)
	cursor = strings.TrimSpace(cursor)
	if s.driver == "pgx" {
		_, err := s.db.ExecContext(ctx, `
UPDATE im_channel_accounts
SET sync_cursor = $4, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = $1 AND channel_type = $2 AND account_id = $3`,
			ownerUserID, ChannelTypeWeixinPersonal, accountID, cursor)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE im_channel_accounts
SET sync_cursor = ?, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = ? AND channel_type = ? AND account_id = ?`,
		cursor, ownerUserID, ChannelTypeWeixinPersonal, accountID)
	return err
}

func (s *ControlService) MarkPersonalWeixinLoginExpired(
	ctx context.Context,
	ownerUserID string,
	accountID string,
	lastError string,
) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	accountID = strings.TrimSpace(accountID)
	lastError = strings.TrimSpace(lastError)
	if s.driver == "pgx" {
		_, err := s.db.ExecContext(ctx, `
UPDATE im_channel_accounts
SET status = $4, last_error = $5, sync_cursor = '', updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = $1 AND channel_type = $2 AND account_id = $3`,
			ownerUserID, ChannelTypeWeixinPersonal, accountID, ChannelConfigStatusError, nullableString(lastError))
		return err
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE im_channel_accounts
SET status = ?, last_error = ?, sync_cursor = '', updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = ? AND channel_type = ? AND account_id = ?`,
		ChannelConfigStatusError, nullableString(lastError), ownerUserID, ChannelTypeWeixinPersonal, accountID)
	return err
}

func (s *ControlService) resetPersonalWeixinCursorWith(
	ctx context.Context,
	store channelStore,
	ownerUserID string,
	accountID string,
) error {
	query := "UPDATE im_channel_accounts SET sync_cursor = '' WHERE owner_user_id = " + s.bind(1) +
		" AND channel_type = " + s.bind(2) + " AND account_id = " + s.bind(3)
	_, err := store.ExecContext(
		ctx,
		query,
		normalizeChannelOwnerUserID(ownerUserID),
		ChannelTypeWeixinPersonal,
		strings.TrimSpace(accountID),
	)
	return err
}
