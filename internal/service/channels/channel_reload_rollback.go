// INPUT: 热重载前的 owner+channel 配置/账号/version 快照与失败后的已提交版本。
// OUTPUT: 仅在版本仍未变化时恢复上一份已知可运行内容，并以新单调版本发布回滚。
// POS: Channel 持久配置与候选 runtime 发布之间的补偿事务边界，禁止版本倒退复活旧 plan。
package channels

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type channelReloadSnapshot struct {
	ownerUserID string
	channelType string
	version     int64
	config      *channelConfigRow
	accounts    []channelAccountRow
}

func (s *ControlService) captureChannelReloadSnapshot(
	ctx context.Context,
	ownerUserID string,
	channelType string,
) (channelReloadSnapshot, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	version, err := s.GetChannelControlVersion(ctx, ownerUserID)
	if err != nil {
		return channelReloadSnapshot{}, err
	}
	configRow, err := s.getChannelConfigRow(ctx, ownerUserID, channelType)
	if err != nil {
		return channelReloadSnapshot{}, err
	}
	accounts, err := s.listChannelAccountRows(ctx, ownerUserID, channelType)
	if err != nil {
		return channelReloadSnapshot{}, err
	}
	return channelReloadSnapshot{
		ownerUserID: ownerUserID,
		channelType: channelType,
		version:     version,
		config:      cloneChannelConfigRow(configRow),
		accounts:    append([]channelAccountRow(nil), accounts...),
	}, nil
}

func (s *ControlService) restoreChannelReloadSnapshot(
	ctx context.Context,
	snapshot channelReloadSnapshot,
	failedVersion int64,
) error {
	if failedVersion <= snapshot.version {
		return fmt.Errorf(
			"invalid failed Channel version %d for previous version %d",
			failedVersion,
			snapshot.version,
		)
	}
	restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(restoreCtx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	versionQuery := "SELECT version FROM channel_control_versions WHERE owner_user_id = " + s.bind(1)
	if s.driver == "pgx" {
		versionQuery += " FOR UPDATE"
	}
	var currentVersion int64
	if err = tx.QueryRowContext(restoreCtx, versionQuery, snapshot.ownerUserID).Scan(&currentVersion); err != nil {
		return err
	}
	if currentVersion != failedVersion {
		return fmt.Errorf(
			"%w: failed reload version=%d current=%d",
			ErrChannelControlVersionConflict,
			failedVersion,
			currentVersion,
		)
	}
	if err = s.deleteChannelAccountRowsWith(
		restoreCtx,
		tx,
		snapshot.ownerUserID,
		snapshot.channelType,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(
		restoreCtx,
		"DELETE FROM im_channel_configs WHERE owner_user_id = "+s.bind(1)+" AND channel_type = "+s.bind(2),
		snapshot.ownerUserID,
		snapshot.channelType,
	); err != nil {
		return err
	}
	if snapshot.config != nil {
		if err = s.insertChannelConfigSnapshot(restoreCtx, tx, *snapshot.config); err != nil {
			return err
		}
	}
	for _, account := range snapshot.accounts {
		if err = s.insertChannelAccountSnapshot(restoreCtx, tx, account); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(
		restoreCtx,
		"UPDATE channel_control_versions SET version = version + 1"+
			", updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = "+s.bind(1)+
			" AND version = "+s.bind(2),
		snapshot.ownerUserID,
		failedVersion,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf(
			"%w: failed reload version=%d changed during rollback",
			ErrChannelControlVersionConflict,
			failedVersion,
		)
	}
	return tx.Commit()
}

func (s *ControlService) insertChannelConfigSnapshot(
	ctx context.Context,
	tx *sql.Tx,
	row channelConfigRow,
) error {
	query := "INSERT INTO im_channel_configs (" +
		"owner_user_id, channel_type, agent_id, status, config_json, " +
		"credentials_encrypted, last_error, created_at, updated_at" +
		") VALUES (" + s.bindList(9) + ")"
	_, err := tx.ExecContext(
		ctx,
		query,
		row.OwnerUserID,
		row.ChannelType,
		row.AgentID,
		row.Status,
		row.ConfigJSON,
		nullStringArgument(row.CredentialsEncrypted),
		nullStringArgument(row.LastError),
		row.CreatedAt,
		row.UpdatedAt,
	)
	return err
}

func (s *ControlService) insertChannelAccountSnapshot(
	ctx context.Context,
	tx *sql.Tx,
	row channelAccountRow,
) error {
	query := "INSERT INTO im_channel_accounts (" +
		"owner_user_id, channel_type, account_id, user_id, status, config_json, " +
		"credentials_encrypted, last_error, sync_cursor, created_at, updated_at" +
		") VALUES (" + s.bindList(11) + ")"
	_, err := tx.ExecContext(
		ctx,
		query,
		row.OwnerUserID,
		row.ChannelType,
		row.AccountID,
		row.UserID,
		row.Status,
		row.ConfigJSON,
		nullStringArgument(row.CredentialsEncrypted),
		nullStringArgument(row.LastError),
		row.SyncCursor,
		row.CreatedAt,
		row.UpdatedAt,
	)
	return err
}

func nullStringArgument(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func cloneChannelConfigRow(value *channelConfigRow) *channelConfigRow {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
