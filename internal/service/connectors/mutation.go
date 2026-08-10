// INPUT: owner、Connector、可选期望版本与同一事务内的持久化变更。
// OUTPUT: 进程内串行、数据库 CAS、单调版本推进或明确的并发冲突。
// POS: Connector connection、OAuth client 与配置控制面的共同写入边界。
package connectors

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrConfigurationConflict 表示 Connector 配置已在计划之后变化。
var ErrConfigurationConflict = errors.New("Connector 配置版本已变化")

func (s *Service) mutateConnector(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
	expectedVersion *int64,
	mutation func(*sql.Tx) error,
) (int64, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	connectorID = strings.TrimSpace(connectorID)
	if ownerUserID == "" || connectorID == "" {
		return 0, errors.New("Connector 配置写入缺少 owner 或 connector_id")
	}
	if expectedVersion != nil && *expectedVersion <= 0 {
		return 0, errors.New("Connector expected configuration version 必须大于 0")
	}

	unlock := s.lockConnectorMutation(ownerUserID, connectorID)
	defer unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	insert := `
INSERT INTO connector_configuration_versions (owner_user_id, connector_id, version)
VALUES (?, ?, 1)
ON CONFLICT(owner_user_id, connector_id) DO NOTHING`
	if s.driver == "pgx" {
		insert = `
INSERT INTO connector_configuration_versions (owner_user_id, connector_id, version)
VALUES ($1, $2, 1)
ON CONFLICT(owner_user_id, connector_id) DO NOTHING`
	}
	if _, err = tx.ExecContext(ctx, insert, ownerUserID, connectorID); err != nil {
		return 0, err
	}

	update := fmt.Sprintf(
		"UPDATE connector_configuration_versions SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = %s AND connector_id = %s",
		s.bind(1),
		s.bind(2),
	)
	args := []any{ownerUserID, connectorID}
	if expectedVersion != nil {
		update += fmt.Sprintf(" AND version = %s", s.bind(3))
		args = append(args, *expectedVersion)
	}
	result, err := tx.ExecContext(ctx, update, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected != 1 {
		return 0, fmt.Errorf(
			"%w：connector=%s expected=%d",
			ErrConfigurationConflict,
			connectorID,
			valueOrZero(expectedVersion),
		)
	}
	if err = mutation(tx); err != nil {
		return 0, err
	}

	var version int64
	query := fmt.Sprintf(
		"SELECT version FROM connector_configuration_versions WHERE owner_user_id = %s AND connector_id = %s",
		s.bind(1),
		s.bind(2),
	)
	if err = tx.QueryRowContext(ctx, query, ownerUserID, connectorID).Scan(&version); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return version, nil
}

func (s *Service) lockConnectorMutation(ownerUserID, connectorID string) func() {
	key := strings.TrimSpace(ownerUserID) + "\x00" + strings.TrimSpace(connectorID)
	value, _ := s.mutations.LoadOrStore(key, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
