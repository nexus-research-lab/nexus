// INPUT: owner-scoped Agent 创建 request identity、intent digest、lease claim 与完整 Agent 记录。
// OUTPUT: 持久 reservation、并发 claim、Agent+Profile+Runtime 原子提交与删除墓碑。
// POS: Agent 创建幂等性的领域仓储；HTTP 诊断 ID 和完整请求永不进入此表。
package agentrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrCreationClaimLost 表示调用方已不再持有当前 pending reservation。
var ErrCreationClaimLost = errors.New("agent creation claim lost")

// ClaimAgentCreation 原子创建 reservation，或在旧 lease 到期后接管同一创建请求。
func (r *SQLRepository) ClaimAgentCreation(
	ctx context.Context,
	candidate CreationRequestRecord,
	nowMS int64,
) (*CreationRequestRecord, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO agent_creation_requests (
    owner_user_id, creation_request_id, intent_digest, agent_id, workspace_path,
    status, stage, claim_token, lease_expires_at_ms, failure_code
) VALUES (%s)
ON CONFLICT(owner_user_id, creation_request_id) DO NOTHING`, r.dialect.BindList(10)),
		strings.TrimSpace(candidate.OwnerUserID),
		strings.TrimSpace(candidate.CreationRequestID),
		strings.TrimSpace(candidate.IntentDigest),
		strings.TrimSpace(candidate.AgentID),
		strings.TrimSpace(candidate.WorkspacePath),
		CreationRequestPending,
		CreationRequestReserved,
		strings.TrimSpace(candidate.ClaimToken),
		candidate.LeaseExpiresAtMS,
		"",
	)
	if err != nil {
		return nil, false, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	claimed := inserted == 1
	current, err := getAgentCreationRequestTx(
		ctx,
		tx,
		r.dialect.Bind,
		candidate.OwnerUserID,
		candidate.CreationRequestID,
	)
	if err != nil {
		return nil, false, err
	}

	if !claimed && current.Status == CreationRequestPending &&
		current.IntentDigest == strings.TrimSpace(candidate.IntentDigest) &&
		current.LeaseExpiresAtMS <= nowMS {
		claimResult, claimErr := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE agent_creation_requests
SET claim_token = %s, lease_expires_at_ms = %s, updated_at = %s
WHERE owner_user_id = %s AND creation_request_id = %s
  AND status = 'pending' AND intent_digest = %s AND lease_expires_at_ms <= %s`,
			r.dialect.Bind(1),
			r.dialect.Bind(2),
			r.dialect.CurrentTimestamp(),
			r.dialect.Bind(3),
			r.dialect.Bind(4),
			r.dialect.Bind(5),
			r.dialect.Bind(6),
		),
			strings.TrimSpace(candidate.ClaimToken),
			candidate.LeaseExpiresAtMS,
			strings.TrimSpace(candidate.OwnerUserID),
			strings.TrimSpace(candidate.CreationRequestID),
			strings.TrimSpace(candidate.IntentDigest),
			nowMS,
		)
		if claimErr != nil {
			return nil, false, claimErr
		}
		affected, rowsErr := claimResult.RowsAffected()
		if rowsErr != nil {
			return nil, false, rowsErr
		}
		claimed = affected == 1
		if claimed {
			current, err = getAgentCreationRequestTx(
				ctx,
				tx,
				r.dialect.Bind,
				candidate.OwnerUserID,
				candidate.CreationRequestID,
			)
			if err != nil {
				return nil, false, err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return current, claimed, nil
}

// GetAgentCreationRequest 按 exact owner/request identity 返回领域回执。
func (r *SQLRepository) GetAgentCreationRequest(
	ctx context.Context,
	ownerUserID string,
	creationRequestID string,
) (*CreationRequestRecord, error) {
	row := r.db.QueryRowContext(ctx, fmt.Sprintf(`
SELECT owner_user_id, creation_request_id, intent_digest, agent_id, workspace_path,
       status, stage, COALESCE(claim_token, ''), lease_expires_at_ms, failure_code
FROM agent_creation_requests
WHERE owner_user_id = %s AND creation_request_id = %s`,
		r.dialect.Bind(1),
		r.dialect.Bind(2),
	), strings.TrimSpace(ownerUserID), strings.TrimSpace(creationRequestID))
	item, err := scanAgentCreationRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// MarkAgentCreationWorkspacePrepared 只在 exact claim 仍有效时推进可重入 workspace 阶段。
func (r *SQLRepository) MarkAgentCreationWorkspacePrepared(
	ctx context.Context,
	claim CreationRequestRecord,
) (bool, error) {
	result, err := r.db.ExecContext(ctx, fmt.Sprintf(`
UPDATE agent_creation_requests
SET stage = 'workspace_prepared', updated_at = %s
WHERE owner_user_id = %s AND creation_request_id = %s AND intent_digest = %s
  AND agent_id = %s AND status = 'pending' AND stage = 'reserved' AND claim_token = %s`,
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(1),
		r.dialect.Bind(2),
		r.dialect.Bind(3),
		r.dialect.Bind(4),
		r.dialect.Bind(5),
	),
		strings.TrimSpace(claim.OwnerUserID),
		strings.TrimSpace(claim.CreationRequestID),
		strings.TrimSpace(claim.IntentDigest),
		strings.TrimSpace(claim.AgentID),
		strings.TrimSpace(claim.ClaimToken),
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

// CommitAgentCreation 在调用方仍持有 exact claim 时，原子提交 Agent 与回执。
func (r *SQLRepository) CommitAgentCreation(
	ctx context.Context,
	claim CreationRequestRecord,
	record CreateRecord,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	claimResult, err := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE agent_creation_requests
SET status = 'committed', claim_token = NULL, lease_expires_at_ms = 0,
    failure_code = '', updated_at = %s
WHERE owner_user_id = %s AND creation_request_id = %s AND intent_digest = %s
  AND agent_id = %s AND status = 'pending' AND stage = 'workspace_prepared' AND claim_token = %s`,
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(1),
		r.dialect.Bind(2),
		r.dialect.Bind(3),
		r.dialect.Bind(4),
		r.dialect.Bind(5),
	),
		strings.TrimSpace(claim.OwnerUserID),
		strings.TrimSpace(claim.CreationRequestID),
		strings.TrimSpace(claim.IntentDigest),
		strings.TrimSpace(claim.AgentID),
		strings.TrimSpace(claim.ClaimToken),
	)
	if err != nil {
		return err
	}
	affected, err := claimResult.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrCreationClaimLost
	}
	if err = r.insertAgent(ctx, tx, record); err != nil {
		return err
	}
	return tx.Commit()
}

// FailAgentCreation 只在 exact claim 仍有效时写入终态失败墓碑。
func (r *SQLRepository) FailAgentCreation(
	ctx context.Context,
	claim CreationRequestRecord,
	failureCode string,
) (bool, error) {
	result, err := r.db.ExecContext(ctx, fmt.Sprintf(`
UPDATE agent_creation_requests
SET status = 'failed', claim_token = NULL, lease_expires_at_ms = 0,
    failure_code = %s, updated_at = %s
WHERE owner_user_id = %s AND creation_request_id = %s AND intent_digest = %s
  AND agent_id = %s AND status = 'pending' AND claim_token = %s`,
		r.dialect.Bind(1),
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(2),
		r.dialect.Bind(3),
		r.dialect.Bind(4),
		r.dialect.Bind(5),
		r.dialect.Bind(6),
	),
		strings.TrimSpace(failureCode),
		strings.TrimSpace(claim.OwnerUserID),
		strings.TrimSpace(claim.CreationRequestID),
		strings.TrimSpace(claim.IntentDigest),
		strings.TrimSpace(claim.AgentID),
		strings.TrimSpace(claim.ClaimToken),
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func getAgentCreationRequestTx(
	ctx context.Context,
	tx *sql.Tx,
	bind func(int) string,
	ownerUserID string,
	creationRequestID string,
) (*CreationRequestRecord, error) {
	row := tx.QueryRowContext(ctx, fmt.Sprintf(`
SELECT owner_user_id, creation_request_id, intent_digest, agent_id, workspace_path,
       status, stage, COALESCE(claim_token, ''), lease_expires_at_ms, failure_code
FROM agent_creation_requests
WHERE owner_user_id = %s AND creation_request_id = %s`, bind(1), bind(2)),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(creationRequestID),
	)
	item, err := scanAgentCreationRequest(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAgentCreationRequest(scanner Scanner) (CreationRequestRecord, error) {
	var item CreationRequestRecord
	err := scanner.Scan(
		&item.OwnerUserID,
		&item.CreationRequestID,
		&item.IntentDigest,
		&item.AgentID,
		&item.WorkspacePath,
		&item.Status,
		&item.Stage,
		&item.ClaimToken,
		&item.LeaseExpiresAtMS,
		&item.FailureCode,
	)
	return item, err
}

func (r *SQLRepository) markAgentCreationRequestsDeleted(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	agentID string,
) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE agent_creation_requests
SET status = 'deleted', claim_token = NULL, lease_expires_at_ms = 0,
    failure_code = '', updated_at = %s
WHERE owner_user_id = %s AND agent_id = %s AND status = 'committed'`,
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(1),
		r.dialect.Bind(2),
	), strings.TrimSpace(ownerUserID), strings.TrimSpace(agentID))
	return err
}
