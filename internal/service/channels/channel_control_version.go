// INPUT: owner 作用域、可选 expected version 与同一事务内的 Channel 控制写入。
// OUTPUT: 持久单调版本 CAS；数据写入和版本推进共同提交或回滚，失败保留提交前或结果未知证据。
// POS: Channel 配置、账号、登录与 pairing 控制状态的数据库并发真相源。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrChannelControlVersionConflict = errors.New("channel control version conflict")

type channelStore interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *ControlService) GetChannelControlVersion(ctx context.Context, ownerUserID string) (int64, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	query := "SELECT version FROM channel_control_versions WHERE owner_user_id = " + s.bind(1)
	var version int64
	err := s.db.QueryRowContext(ctx, query, ownerUserID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	if version < 1 {
		return 0, fmt.Errorf("invalid channel control version for owner %s: %d", ownerUserID, version)
	}
	return version, nil
}

func (s *ControlService) withChannelControlMutation(
	ctx context.Context,
	ownerUserID string,
	expectedVersion int64,
	mutate func(*sql.Tx) error,
) (int64, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	if expectedVersion < 0 {
		return 0, fmt.Errorf("expected channel control version must not be negative")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	defer func() { _ = tx.Rollback() }()

	insertQuery := `
INSERT INTO channel_control_versions (owner_user_id, version, updated_at)
VALUES (` + s.bind(1) + `, 1, CURRENT_TIMESTAMP)
ON CONFLICT (owner_user_id) DO NOTHING`
	if _, err = tx.ExecContext(ctx, insertQuery, ownerUserID); err != nil {
		return 0, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	versionQuery := "SELECT version FROM channel_control_versions WHERE owner_user_id = " + s.bind(1)
	if s.driver == "pgx" {
		versionQuery += " FOR UPDATE"
	}
	var currentVersion int64
	if err = tx.QueryRowContext(ctx, versionQuery, ownerUserID).Scan(&currentVersion); err != nil {
		return 0, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	if expectedVersion > 0 && currentVersion != expectedVersion {
		return 0, channelControlMutationFailure(ControlMutationNotApplied, fmt.Errorf(
			"%w: expected=%d current=%d",
			ErrChannelControlVersionConflict,
			expectedVersion,
			currentVersion,
		))
	}
	updateQuery := `
UPDATE channel_control_versions
SET version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = ` + s.bind(1) + ` AND version = ` + s.bind(2)
	result, err := tx.ExecContext(ctx, updateQuery, ownerUserID, currentVersion)
	if err != nil {
		return 0, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	if affected != 1 {
		return 0, channelControlMutationFailure(ControlMutationNotApplied, fmt.Errorf(
			"%w: expected=%d current changed concurrently",
			ErrChannelControlVersionConflict,
			currentVersion,
		))
	}
	if mutate != nil {
		if err = mutate(tx); err != nil {
			return 0, channelControlMutationFailure(ControlMutationNotApplied, err)
		}
	}
	if err = tx.Commit(); err != nil {
		// database/sql cannot prove whether a commit error reached durable storage.
		// The caller must reconcile the exact Channel aggregate before another write.
		return 0, channelControlMutationFailure(ControlMutationUnknown, err)
	}
	return currentVersion + 1, nil
}

func normalizeExpectedChannelControlVersion(expectedVersion int64) (int64, error) {
	if expectedVersion <= 0 {
		return 0, invalidChannelControl(errors.New("channel control mutation requires a positive expected version"))
	}
	return expectedVersion, nil
}

func channelControlVersionError(expectedVersion int64, err error) error {
	if !errors.Is(err, ErrChannelControlVersionConflict) {
		return err
	}
	return fmt.Errorf(
		"Channel 配置版本已变化（expected=%d）；请重新 inspect/plan 后核对: %w",
		expectedVersion,
		err,
	)
}

func normalizedChannelOwner(ownerUserID string) string {
	return normalizeChannelOwnerUserID(strings.TrimSpace(ownerUserID))
}
